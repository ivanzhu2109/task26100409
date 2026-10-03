// perf 是 domaincand 库的性能验证入口：构建 30 万标签（约 500 万字符）
// 的词表，连续执行 200 个查询，报告总耗时与进程峰值内存。
package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"syscall"
	"time"

	"domaincand"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789-"

func randString(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// buildLabels 生成 30 万个标签：一半为分散随机标签，一半为共享
// 长前后缀的密集标签，总长度控制在 600 万字符以内。
func buildLabels(rng *rand.Rand) []string {
	labels := make([]string, 0, 300000)
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			labels = append(labels, s)
		}
	}
	// 分散标签：长度 4-16 的随机串。
	for len(labels) < 150000 {
		add(randString(rng, 4+rng.Intn(13)))
	}
	// 密集标签：长公共前缀 + 短变化中段 + 长公共后缀。
	prefixes := []string{"cloudserviceplt", "enterprise-data", "globalmarketp"}
	suffixes := []string{"solutions", "systems", "-online"}
	for len(labels) < 300000 {
		p := prefixes[rng.Intn(len(prefixes))]
		s := suffixes[rng.Intn(len(suffixes))]
		add(p + randString(rng, 2+rng.Intn(3)) + s)
	}
	return labels
}

func peakRSSMiB() float64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / (1 << 20) // darwin 上 Maxrss 单位为字节
}

func main() {
	rng := rand.New(rand.NewSource(42))
	labels := buildLabels(rng)
	total := 0
	for _, s := range labels {
		total += len(s)
	}
	fmt.Printf("标签数: %d, 标签总长度: %d 字符, 候选域名数: %d\n",
		len(labels), total, len(labels)*len(domaincand.TLDs))

	start := time.Now()
	lib, err := domaincand.New(labels)
	if err != nil {
		panic(err)
	}
	fmt.Printf("建库耗时: %v\n", time.Since(start).Round(time.Millisecond))

	// 随机占用 5% 的候选域名。
	for i := 0; i < 150000; i++ {
		d := labels[rng.Intn(len(labels))] + "." + domaincand.TLDs[rng.Intn(len(domaincand.TLDs))]
		if err := lib.SetOccupied(d, true); err != nil {
			panic(err)
		}
	}

	// 200 个查询：一半命中（词表标签做 0-2 次编辑），一半无匹配（随机串）。
	queries := make([]string, 0, 200)
	for i := 0; i < 100; i++ {
		s := labels[rng.Intn(len(labels))]
		b := []byte(s)
		for e := 0; e < rng.Intn(3); e++ {
			pos := rng.Intn(len(b))
			b[pos] = charset[rng.Intn(len(charset))]
		}
		queries = append(queries, string(b))
	}
	for i := 0; i < 100; i++ {
		queries = append(queries, randString(rng, 8+rng.Intn(10)))
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("查询前堆内存: %.1f MiB\n", float64(mem.HeapInuse)/(1<<20))

	start = time.Now()
	hits := 0
	for i, q := range queries {
		tld := ""
		if i%4 == 0 {
			tld = domaincand.TLDs[rng.Intn(len(domaincand.TLDs))]
		}
		res, err := lib.Query(q, tld)
		if err != nil {
			panic(err)
		}
		hits += len(res)
	}
	elapsed := time.Since(start)
	fmt.Printf("200 个查询总耗时: %v (平均 %v/查询), 返回候选总数: %d\n",
		elapsed.Round(time.Millisecond), (elapsed / 200).Round(time.Microsecond), hits)
	fmt.Printf("进程峰值内存 (MaxRSS): %.1f MiB\n", peakRSSMiB())
}
