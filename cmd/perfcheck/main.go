// Command perfcheck builds a 300k-label index (3M candidate domains) and
// runs 200 distinct queries, reporting wall time and peak memory against the
// required limits (200 queries <= 20s, peak memory <= 512 MiB).
package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"syscall"
	"time"

	"domaincand"
)

const (
	numLabels    = 300_000
	numQueries   = 200
	queryBudget  = 20 * time.Second
	memoryBudget = 512 * 1024 * 1024 // 512 MiB
)

func maxRSSBytes() uint64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	if runtime.GOOS == "darwin" {
		return uint64(ru.Maxrss) // bytes on macOS
	}
	return uint64(ru.Maxrss) * 1024 // kilobytes on Linux
}

func buildVocabulary(rng *rand.Rand) []string {
	labels := make([]string, 0, numLabels)
	seen := map[string]bool{}
	add := func(s string) {
		if len(s) >= 1 && len(s) <= 63 && !seen[s] {
			seen[s] = true
			labels = append(labels, s)
		}
	}
	// Dense half: long shared prefixes and suffixes.
	for len(labels) < numLabels/2 {
		prefix := fmt.Sprintf("sharedprefix%04d", rng.Intn(1500))
		s := prefix
		for len(s) < 16+rng.Intn(10) {
			s += string(rune('a' + rng.Intn(8)))
		}
		add(s)
	}
	// Scattered half: random labels.
	for len(labels) < numLabels {
		n := 6 + rng.Intn(18)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('a' + rng.Intn(26))
		}
		add(string(b))
	}
	return labels
}

func buildQueries(rng *rand.Rand, labels []string) []string {
	queries := make([]string, 0, numQueries)
	seen := map[string]bool{}
	add := func(s string) {
		if len(s) >= 1 && len(s) <= 63 && !seen[s] && len(queries) < numQueries {
			seen[s] = true
			queries = append(queries, s)
		}
	}
	mutate := func(s string) string {
		b := []byte(s)
		switch rng.Intn(4) {
		case 0: // substitute
			b[rng.Intn(len(b))] = byte('a' + rng.Intn(26))
		case 1: // swap adjacent
			i := rng.Intn(len(b) - 1)
			b[i], b[i+1] = b[i+1], b[i]
		case 2: // delete
			i := rng.Intn(len(b))
			b = append(b[:i], b[i+1:]...)
		case 3: // insert
			i := rng.Intn(len(b) + 1)
			b = append(b[:i], append([]byte{byte('a' + rng.Intn(26))}, b[i:]...)...)
		}
		return string(b)
	}
	// ~1/3 exact hits.
	for len(queries) < numQueries/3 {
		add(labels[rng.Intn(len(labels))])
	}
	// ~1/3 near matches (1-2 edits away from a vocabulary label).
	for len(queries) < 2*numQueries/3 {
		s := mutate(labels[rng.Intn(len(labels))])
		if rng.Float64() < 0.5 {
			s = mutate(s)
		}
		add(s)
	}
	// ~1/3 far misses.
	for len(queries) < numQueries {
		n := 8 + rng.Intn(12)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('a' + rng.Intn(26))
		}
		add(string(b))
	}
	return queries
}

func mib(b uint64) float64 { return float64(b) / (1024 * 1024) }

func main() {
	rng := rand.New(rand.NewSource(20261003))
	labels := buildVocabulary(rng)
	total := 0
	for _, l := range labels {
		total += len(l)
	}
	fmt.Printf("vocabulary: %d labels, %d total chars, %d candidate domains\n",
		len(labels), total, len(labels)*len(domaincand.TLDs))

	start := time.Now()
	idx := domaincand.New(labels)
	build := time.Since(start)
	fmt.Printf("build: %v (not counted towards the query budget)\n", build.Round(time.Millisecond))

	// occupy ~10% of domains so results mix both states
	for _, l := range labels {
		for _, tld := range domaincand.TLDs {
			if rng.Float64() < 0.1 {
				idx.SetOccupied(l+"."+tld, true)
			}
		}
	}

	queries := buildQueries(rng, labels)
	start = time.Now()
	totalResults := 0
	occupiedSeen := 0
	for _, q := range queries {
		res := idx.Query(q, "")
		totalResults += len(res)
		for _, c := range res {
			if c.Occupied {
				occupiedSeen++
			}
		}
	}
	elapsed := time.Since(start)
	fmt.Printf("queries: %d distinct queries in %v (%.1f ms/query), %d results, %d occupied\n",
		len(queries), elapsed.Round(time.Millisecond),
		float64(elapsed.Microseconds())/1000.0/float64(len(queries)),
		totalResults, occupiedSeen)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("memory: heap in use %.1f MiB, peak RSS %.1f MiB (budget %d MiB)\n",
		mib(ms.HeapAlloc), mib(maxRSSBytes()), memoryBudget/(1024*1024))

	ok := true
	if elapsed > queryBudget {
		fmt.Printf("FAIL: query time %v exceeds budget %v\n", elapsed, queryBudget)
		ok = false
	}
	if maxRSSBytes() > memoryBudget {
		fmt.Printf("FAIL: peak RSS %.1f MiB exceeds budget %d MiB\n", mib(maxRSSBytes()), memoryBudget/(1024*1024))
		ok = false
	}
	if ok {
		fmt.Println("PASS: within time and memory budgets")
	}
}
