package domaincand

import "testing"

func TestDamerauLevenshteinExamples(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"ca", "abc", 2},  // swap to "ac", then insert "b"
		{"abc", "acb", 1}, // adjacent swap
		{"abcd", "abdc", 1},
		{"aaa", "aaaa", 1}, // repeated characters
		{"aaaa", "aa", 2},
		{"abcd", "badc", 2}, // two swaps
		{"abcdef", "abcfed", 2},
		{"kitten", "sitting", 3},
		{"abcdef", "azced", 3},
		{"abcd", "dcba", 3}, // sub a->d, sub d->a, swap bc
	}
	for _, c := range cases {
		if got := damerauLevenshtein(c.a, c.b); got != c.want {
			t.Errorf("damerauLevenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := damerauLevenshtein(c.b, c.a); got != c.want {
			t.Errorf("damerauLevenshtein(%q, %q) = %d, want %d", c.b, c.a, got, c.want)
		}
	}
}

// naiveEditDistance BFSes the edit graph; it is slow but obviously correct
// and serves as an independent oracle for short strings.
func naiveEditDistance(a, b string) int {
	if a == b {
		return 0
	}
	const alphabet = "abc"
	type state struct {
		s string
		d int
	}
	seen := map[string]bool{a: true}
	queue := []state{{a, 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		neighbors := func(s string) []string {
			var out []string
			// deletions
			for i := 0; i < len(s); i++ {
				out = append(out, s[:i]+s[i+1:])
			}
			// insertions
			for i := 0; i <= len(s); i++ {
				for _, c := range alphabet {
					out = append(out, s[:i]+string(c)+s[i:])
				}
			}
			// substitutions
			for i := 0; i < len(s); i++ {
				for _, c := range alphabet {
					if byte(c) != s[i] {
						out = append(out, s[:i]+string(c)+s[i+1:])
					}
				}
			}
			// adjacent transpositions
			for i := 0; i+1 < len(s); i++ {
				if s[i] != s[i+1] {
					out = append(out, s[:i]+string(s[i+1])+string(s[i])+s[i+2:])
				}
			}
			return out
		}
		for _, n := range neighbors(cur.s) {
			if n == b {
				return cur.d + 1
			}
			if !seen[n] && len(n) <= max(len(a), len(b))+2 {
				seen[n] = true
				queue = append(queue, state{n, cur.d + 1})
			}
		}
	}
	return -1
}

func allStrings(alphabet string, maxLen int) []string {
	var out []string
	var rec func(s string)
	rec = func(s string) {
		out = append(out, s)
		if len(s) < maxLen {
			for _, c := range alphabet {
				rec(s + string(c))
			}
		}
	}
	rec("")
	return out
}

func TestDamerauLevenshteinAgainstOracle(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive oracle comparison is slow")
	}
	strs := allStrings("abc", 4)
	for _, a := range strs {
		for _, b := range strs {
			want := naiveEditDistance(a, b)
			if got := damerauLevenshtein(a, b); got != want {
				t.Fatalf("damerauLevenshtein(%q, %q) = %d, oracle says %d", a, b, got, want)
			}
			for limit := 0; limit <= 3; limit++ {
				got := damerauLevenshteinAtMost(a, b, limit)
				if want <= limit && got != want {
					t.Fatalf("damerauLevenshteinAtMost(%q, %q, %d) = %d, want %d", a, b, limit, got, want)
				}
				if want > limit && got <= limit {
					t.Fatalf("damerauLevenshteinAtMost(%q, %q, %d) = %d, want > %d", a, b, limit, got, limit)
				}
			}
		}
	}
}
