package domaincand

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// bruteForce 对全部 标签×TLD 候选逐一计算参考距离，按完整排序规则取前 50。
func bruteForce(t *testing.T, l *Library, label, tld string) []Candidate {
	t.Helper()
	tldFirst, tldLast := 0, len(TLDs)
	if tld != "" {
		tldFirst, tldLast = tldIndex(tld), tldIndex(tld)+1
	}
	var all []Candidate
	for li, base := range l.labels {
		d := refDL(label, base)
		if d > maxDistance {
			continue
		}
		for ti := tldFirst; ti < tldLast; ti++ {
			all = append(all, Candidate{
				Domain:   base + "." + TLDs[ti],
				Distance: d,
				Occupied: l.isOccupied(int32(li), ti),
			})
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
	if len(all) > maxResults {
		all = all[:maxResults]
	}
	return all
}

func mustNew(t *testing.T, labels []string) *Library {
	t.Helper()
	l, err := New(labels)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func TestQuerySpecExample(t *testing.T) {
	l := mustNew(t, []string{"abc", "abd", "xyz"})
	got, err := l.Query("ca", "com")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Domain != "abc.com" || got[0].Distance != 2 || got[0].Occupied {
		t.Fatalf("got %+v", got)
	}
}

func TestQueryDistanceBoundaries(t *testing.T) {
	// 包含距离恰好 2 与恰好 3 的边界。
	l := mustNew(t, []string{"test", "tset", "testx", "txst", "texx", "testxy", "abcdef"})
	got, err := l.Query("test", "com")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"test.com":   0, // 相同
		"tset.com":   1, // 交换
		"testx.com":  1, // 插入
		"txst.com":   1, // 替换
		"texx.com":   2, // 两次替换，距离恰好 2
		"testxy.com": 2, // 距离恰好 2，须入选
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates: %+v", len(got), got)
	}
	for _, c := range got {
		d, ok := want[c.Domain]
		if !ok {
			t.Fatalf("unexpected candidate %+v", c)
		}
		if c.Distance != d {
			t.Fatalf("%s distance=%d, want %d", c.Domain, c.Distance, d)
		}
	}
	// "abcdef" 距离 6，"testxy" 距离 2；再确认距离 3 的被排除。
	l2 := mustNew(t, []string{"abcd", "abcx", "abcxyz"})
	got2, _ := l2.Query("abc", "com")
	for _, c := range got2 {
		if c.Domain == "abcxyz.com" { // 距离 3，不得入选
			t.Fatalf("distance-3 candidate returned: %+v", c)
		}
	}
}

func TestQueryTransposeThenInsert(t *testing.T) {
	// “ca” -> 交换得 “ac” -> 插入 “b” 得 “abc”，距离 2。
	l := mustNew(t, []string{"abc"})
	for _, tld := range []string{"", "com", "xyz"} {
		got, err := l.Query("ca", tld)
		if err != nil {
			t.Fatal(err)
		}
		wantN := 1
		if tld == "" {
			wantN = len(TLDs)
		}
		if len(got) != wantN {
			t.Fatalf("tld=%q: got %d candidates, want %d", tld, len(got), wantN)
		}
		for _, c := range got {
			if c.Distance != 2 {
				t.Fatalf("%s distance=%d, want 2", c.Domain, c.Distance)
			}
		}
	}
}

func TestQueryAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	alphabet := "abc-"
	labelSet := map[string]bool{}
	var labels []string
	for len(labels) < 300 {
		s := randLabel(rng, alphabet, 6)
		if !labelSet[s] {
			labelSet[s] = true
			labels = append(labels, s)
		}
	}
	l := mustNew(t, labels)
	// 随机占用一批域名。
	for i := 0; i < 200; i++ {
		d := labels[rng.Intn(len(labels))] + "." + TLDs[rng.Intn(len(TLDs))]
		if err := l.SetOccupied(d, rng.Intn(2) == 0); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 500; i++ {
		q := randLabel(rng, alphabet, 6)
		tld := ""
		if rng.Intn(3) == 0 {
			tld = TLDs[rng.Intn(len(TLDs))]
		}
		got, err := l.Query(q, tld)
		if err != nil {
			t.Fatal(err)
		}
		want := bruteForce(t, l, q, tld)
		if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
			t.Fatalf("query %q tld %q:\n got %+v\nwant %+v", q, tld, got, want)
		}
	}
}

func TestQueryLoadOrderIndependent(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	labels := []string{"alpha", "alpa", "beta", "bet", "gamma", "gama", "delta"}
	shuffled := append([]string(nil), labels...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	a := mustNew(t, labels)
	b := mustNew(t, shuffled)
	ga, _ := a.Query("alha", "")
	gb, _ := b.Query("alha", "")
	if fmt.Sprintf("%+v", ga) != fmt.Sprintf("%+v", gb) {
		t.Fatalf("load order changed result:\n%+v\n%+v", ga, gb)
	}
}

func TestQueryTop50AndOccupancyReorder(t *testing.T) {
	// 6 个与 "test" 距离 1 的标签 × 10 个 TLD = 60 个同距离候选。
	labels := []string{"test", "testa", "testb", "testc", "testd", "teste"}
	l := mustNew(t, labels)
	sortedTLDs := append([]string(nil), TLDs...)
	sort.Strings(sortedTLDs)
	got, err := l.Query("test", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxResults {
		t.Fatalf("got %d candidates, want %d", len(got), maxResults)
	}
	// 前 10 个应为距离 0 的 test.*，按域名字节序（TLD 字典序）排列。
	for i := 0; i < len(TLDs); i++ {
		if got[i].Distance != 0 || got[i].Domain != "test."+sortedTLDs[i] {
			t.Fatalf("slot %d: %+v", i, got[i])
		}
	}
	// 距离 1 的候选按域名字节序取前 40 个（testa~testd 各 10 个 TLD），
	// 第 51 项原本是 "teste.app"。
	last := got[maxResults-1].Domain
	if last != "testd.xyz" {
		t.Fatalf("last=%q, want testd.xyz", last)
	}
	// 占用原本未入选的 "teste.xyz"：它落到距离 1 组末尾，仍不进前 50；
	// 再占用原本入选的 "testa.app"：它落到全部可用距离 1 候选之后、
	// 退出前 50，原第 51 项 "teste.app" 进入前 50。
	if err := l.SetOccupied("testa.app", true); err != nil {
		t.Fatal(err)
	}
	if err := l.SetOccupied("teste.xyz", true); err != nil {
		t.Fatal(err)
	}
	got2, _ := l.Query("test", "")
	if len(got2) != maxResults {
		t.Fatalf("got %d candidates, want %d", len(got2), maxResults)
	}
	if got2[maxResults-1].Domain != "teste.app" {
		t.Fatalf("after reorder last=%q, want teste.app", got2[maxResults-1].Domain)
	}
	for _, c := range got2 {
		if c.Domain == "testa.app" || c.Domain == "teste.xyz" {
			t.Fatalf("occupied %s should fall out of top 50: %+v", c.Domain, c)
		}
	}
	// 解除占用后恢复原有顺序。
	if err := l.SetOccupied("testa.app", false); err != nil {
		t.Fatal(err)
	}
	got3, _ := l.Query("test", "")
	if got3[10].Domain != "testa.app" || got3[maxResults-1].Domain != "testd.xyz" {
		t.Fatalf("unoccupy did not restore order: %+v ... %+v", got3[10], got3[maxResults-1])
	}
	// 与暴力核对保持一致。
	want := bruteForce(t, l, "test", "")
	if fmt.Sprintf("%+v", got3) != fmt.Sprintf("%+v", want) {
		t.Fatalf("mismatch with brute force:\n got %+v\nwant %+v", got3, want)
	}
}

func TestSetOccupiedErrors(t *testing.T) {
	l := mustNew(t, []string{"abc"})
	for _, d := range []string{"abc", "abc.uk", "abd.com", ".com", "abc.com."} {
		if err := l.SetOccupied(d, true); err == nil {
			t.Fatalf("SetOccupied(%q) should fail", d)
		}
	}
	if err := l.SetOccupied("abc.com", true); err != nil {
		t.Fatal(err)
	}
	got, _ := l.Query("abc", "com")
	if len(got) != 1 || !got[0].Occupied {
		t.Fatalf("got %+v", got)
	}
}

func TestNewValidation(t *testing.T) {
	if _, err := New([]string{"abc", "abc"}); err == nil {
		t.Fatal("duplicate labels should fail")
	}
	for _, s := range []string{"", "Abc", "ab_c", "ab.c", "中文"} {
		if _, err := New([]string{s}); err == nil {
			t.Fatalf("invalid label %q should fail", s)
		}
	}
	long := make([]byte, 64)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := New([]string{string(long)}); err == nil {
		t.Fatal("64-char label should fail")
	}
}

func TestQueryInvalidInput(t *testing.T) {
	l := mustNew(t, []string{"abc"})
	if _, err := l.Query("ABC", ""); err == nil {
		t.Fatal("uppercase query should fail")
	}
	if _, err := l.Query("abc", "uk"); err == nil {
		t.Fatal("unsupported tld should fail")
	}
}
