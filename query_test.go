package domaincand

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// bruteForceQuery is the independent reference implementation: it checks
// every candidate domain one by one and applies the full ordering rules.
func bruteForceQuery(x *Index, label, tld string) []Candidate {
	tlds := TLDs
	if tld != "" {
		tlds = []string{tld}
	}
	var all []Candidate
	for _, l := range x.labels {
		d := damerauLevenshtein(l, label)
		if d > maxDistance {
			continue
		}
		for _, t := range tlds {
			dom := l + "." + t
			all = append(all, Candidate{Domain: dom, Distance: d, Occupied: x.Occupied(dom)})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		if a.Occupied != b.Occupied {
			return !a.Occupied
		}
		return a.Domain < b.Domain
	})
	if len(all) > MaxResults {
		all = all[:MaxResults]
	}
	return all
}

func checkQuery(t *testing.T, x *Index, label, tld string) {
	t.Helper()
	got := x.Query(label, tld)
	want := bruteForceQuery(x, label, tld)
	if len(got) != len(want) {
		t.Fatalf("Query(%q, %q): got %d results, want %d\ngot:  %v\nwant: %v",
			label, tld, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Query(%q, %q) result %d: got %+v, want %+v\ngot:  %v\nwant: %v",
				label, tld, i, got[i], want[i], got, want)
		}
	}
	// no duplicates
	seen := map[string]bool{}
	for _, c := range got {
		if seen[c.Domain] {
			t.Fatalf("Query(%q, %q): duplicate domain %q", label, tld, c.Domain)
		}
		seen[c.Domain] = true
	}
}

func TestQueryExhaustiveSmall(t *testing.T) {
	labels := allStrings("ab", 4)[1:] // skip the empty string
	x := New(labels)
	rng := rand.New(rand.NewSource(1))
	// random occupancy on ~20% of domains
	for _, l := range labels {
		for _, tld := range TLDs {
			if rng.Float64() < 0.2 {
				x.SetOccupied(l+"."+tld, true)
			}
		}
	}
	queries := allStrings("ab", 4)
	for _, q := range queries {
		checkQuery(t, x, q, "")
		checkQuery(t, x, q, "com")
		checkQuery(t, x, q, "io")
	}
}

func TestQueryTranspositionVocab(t *testing.T) {
	labels := []string{"ca", "ac", "abc", "acb", "bac", "bca", "cab", "cba",
		"aaa", "aa", "aaaa", "ab", "ba", "aab", "aba", "abb", "abcabc"}
	x := New(labels)
	for _, q := range []string{"ca", "abc", "ac", "aa", "aaa", "abcd", "z", "bac"} {
		checkQuery(t, x, q, "")
		checkQuery(t, x, q, "dev")
	}
}

func TestQueryHyphenAndPrefixLabels(t *testing.T) {
	labels := []string{"ab", "ab-c", "abc", "a-b", "a", "ab0", "ab-", "-ab", "0ab"}
	x := New(labels)
	for _, q := range []string{"ab", "ab-c", "a", "abc"} {
		checkQuery(t, x, q, "")
	}
}

func TestQueryRandomDense(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// dense vocabulary: long shared prefixes + scattered labels
	var labels []string
	seen := map[string]bool{}
	add := func(s string) {
		if len(s) >= 1 && len(s) <= 63 && !seen[s] {
			seen[s] = true
			labels = append(labels, s)
		}
	}
	for i := 0; i < 400; i++ {
		prefix := "sharedprefix" + fmt.Sprint(i%4)
		s := prefix
		for len(s) < 14+rng.Intn(8) {
			s += string(rune('a' + rng.Intn(6)))
		}
		add(s)
	}
	for i := 0; i < 400; i++ {
		n := 1 + rng.Intn(10)
		s := ""
		for len(s) < n {
			s += string(rune('a' + rng.Intn(26)))
		}
		add(s)
	}
	x := New(labels)
	for _, l := range labels {
		for _, tld := range TLDs {
			if rng.Float64() < 0.3 {
				x.SetOccupied(l+"."+tld, true)
			}
		}
	}
	for i := 0; i < 60; i++ {
		var q string
		if i%3 == 0 {
			q = labels[rng.Intn(len(labels))] // exact hit
		} else {
			n := 1 + rng.Intn(12)
			for len(q) < n {
				q += string(rune('a' + rng.Intn(6)))
			}
		}
		checkQuery(t, x, q, "")
		checkQuery(t, x, q, TLDs[rng.Intn(len(TLDs))])
	}
}

func TestQueryOccupancyUpdateReshuffles(t *testing.T) {
	// 6 labels all at distance 1 from query "test0" -> 60 candidates at the
	// same distance, exceeding MaxResults across TLDs.
	labels := []string{"test1", "test2", "test3", "test4", "test5", "test6"}
	x := New(labels)
	q := "test0"

	before := x.Query(q, "")
	if len(before) != MaxResults {
		t.Fatalf("expected %d results, got %d", MaxResults, len(before))
	}
	checkQuery(t, x, q, "")

	// The very first result is test1.app (distance 1, available, smallest
	// domain). Occupying it must push it behind all available distance-1
	// domains, letting a previously unlisted domain in.
	first := before[0].Domain
	if first != "test1.app" {
		t.Fatalf("unexpected first candidate %q", first)
	}
	tail := before[MaxResults-1].Domain
	if !x.SetOccupied(first, true) {
		t.Fatalf("SetOccupied(%q) failed", first)
	}
	after := x.Query(q, "")
	if len(after) != MaxResults {
		t.Fatalf("expected %d results after update, got %d", MaxResults, len(after))
	}
	// 59 available distance-1 domains remain, so the occupied one drops out
	// of the top 50 entirely.
	for _, c := range after {
		if c.Domain == first {
			t.Fatalf("occupied domain %q should have dropped out of top %d", first, MaxResults)
		}
		if c.Occupied {
			t.Fatalf("unexpected occupied domain %q in results", c.Domain)
		}
	}
	// a domain that was ranked out before must now appear
	foundNew := false
	for _, c := range after {
		inside := false
		for _, b := range before {
			if b.Domain == c.Domain {
				inside = true
				break
			}
		}
		if !inside {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatalf("no new domain entered the top %d after occupancy change", MaxResults)
	}
	// full agreement with brute force after the change
	checkQuery(t, x, q, "")
	// occupy everything but 5 domains: the top 50 must list all 5 available
	// first, then occupied ones in domain order.
	var doms []string
	for _, l := range labels {
		for _, tld := range TLDs {
			doms = append(doms, l+"."+tld)
		}
	}
	sort.Strings(doms)
	for _, dom := range doms[5:] { // keep only the 5 smallest available
		x.SetOccupied(dom, true)
	}
	after = x.Query(q, "")
	cut := -1
	for i, c := range after {
		if c.Occupied && cut < 0 {
			cut = i
		}
		if !c.Occupied && cut >= 0 {
			t.Fatalf("available domain %q ranked after occupied ones", c.Domain)
		}
	}
	if cut < 0 {
		t.Fatal("expected some occupied domains in results")
	}
	// Un-occupy a domain currently ranked outside the top 50: it must enter
	// the available group at the front on the very next query.
	rankedOut := ""
	present := map[string]bool{}
	for _, c := range after {
		present[c.Domain] = true
	}
	for _, dom := range doms {
		if !present[dom] {
			rankedOut = dom
			break
		}
	}
	if rankedOut == "" {
		t.Fatal("expected a domain ranked outside the top 50")
	}
	if !x.SetOccupied(rankedOut, false) {
		t.Fatalf("SetOccupied(%q, false) failed", rankedOut)
	}
	after = x.Query(q, "")
	found := false
	for i, c := range after {
		if c.Domain == rankedOut {
			found = true
			if c.Occupied {
				t.Fatalf("domain %q still reported occupied", rankedOut)
			}
		}
		if c.Occupied && !found {
			t.Fatalf("occupied domain %q ranked before un-occupied %q (pos %d)", c.Domain, rankedOut, i)
		}
	}
	if !found {
		t.Fatalf("un-occupied domain %q did not enter the top %d", rankedOut, MaxResults)
	}
	checkQuery(t, x, q, "")
	// un-occupying restores availability
	if !x.SetOccupied(tail, true) || !x.SetOccupied(tail, false) {
		t.Fatalf("SetOccupied toggle failed for %q", tail)
	}
	checkQuery(t, x, q, "")
}

func TestQueryUnknownTLDAndDomain(t *testing.T) {
	x := New([]string{"alpha", "beta"})
	if got := x.Query("alpha", "ninja"); got != nil {
		t.Fatalf("unknown TLD should yield no results, got %v", got)
	}
	if x.SetOccupied("alpha.ninja", true) {
		t.Fatal("SetOccupied accepted unknown TLD")
	}
	if x.SetOccupied("gamma.com", true) {
		t.Fatal("SetOccupied accepted unknown label")
	}
	if x.SetOccupied("nodot", true) {
		t.Fatal("SetOccupied accepted domain without dot")
	}
	if !x.SetOccupied("alpha.com", true) || !x.Occupied("alpha.com") {
		t.Fatal("SetOccupied/Occupied roundtrip failed")
	}
	if x.Occupied("alpha.net") {
		t.Fatal("unrelated domain should be available")
	}
}
