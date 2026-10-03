package domaincand

import (
	"math/rand"
	"testing"
)

// refDL 是无裁剪、无截断的 Damerau-Levenshtein 参考实现，用于对照。
func refDL(a, b string) int {
	n, m := len(a), len(b)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
	}
	for i := 0; i <= n; i++ {
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	da := map[byte]int{}
	for i := 1; i <= n; i++ {
		db := 0
		for j := 1; j <= m; j++ {
			i1 := da[b[j-1]]
			j1 := db
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
				db = j
			}
			v := min3(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i1 > 0 && j1 > 0 {
				if t := d[i1-1][j1-1] + (i - i1 - 1) + 1 + (j - j1 - 1); t < v {
					v = t
				}
			}
			d[i][j] = v
		}
		da[a[i-1]] = i
	}
	return d[n][m]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func TestDlDist2Examples(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 0},   // 完全相同
		{"ca", "abc", 2},    // 交换得 ac，再插入 b
		{"ab", "ba", 1},     // 相邻交换
		{"aaa", "aa", 1},    // 重复字符删除
		{"aa", "aaa", 1},    // 重复字符插入
		{"aabb", "abab", 1}, // 重复字符交换
		{"abcd", "abdc", 1}, // 交换
		{"abc", "abd", 1},   // 替换
		{"abc", "ac", 1},    // 删除
		{"ac", "abc", 1},    // 插入
		{"abc", "xyz", 3},   // 距离 3，超出上限
		{"abc", "axy", 2},   // 距离恰好 2
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
	}
	for _, c := range cases {
		if got := dlDist2(c.a, c.b); got != c.want {
			t.Errorf("dlDist2(%q,%q)=%d, want %d", c.a, c.b, got, c.want)
		}
		if got := refDL(c.a, c.b); got != c.want {
			t.Errorf("refDL(%q,%q)=%d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestDlDist2AgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := "abcde-" // 小字符集更容易产生近距离对
	for i := 0; i < 20000; i++ {
		a := randLabel(rng, alphabet, 8)
		b := randLabel(rng, alphabet, 8)
		want := refDL(a, b)
		got := dlDist2(a, b)
		if want > maxDistance {
			want = maxDistance + 1
		}
		if got != want {
			t.Fatalf("dlDist2(%q,%q)=%d, want %d", a, b, got, want)
		}
	}
}

func randLabel(rng *rand.Rand, alphabet string, maxLen int) string {
	n := 1 + rng.Intn(maxLen)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}
