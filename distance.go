package domaincand

// damerauLevenshtein returns the full Damerau-Levenshtein distance between a
// and b: the minimum number of insertions, deletions, substitutions and
// adjacent transpositions needed to turn a into b. Characters may take part
// in multiple edits (Lowrance-Wagner algorithm).
func damerauLevenshtein(a, b string) int {
	return damerauLevenshteinAtMost(a, b, len(a)+len(b))
}

// damerauLevenshteinAtMost returns the full Damerau-Levenshtein distance
// between a and b if it is at most limit, and limit+1 otherwise.
func damerauLevenshteinAtMost(a, b string, limit int) int {
	la, lb := len(a), len(b)
	if la == 0 {
		if lb <= limit {
			return lb
		}
		return limit + 1
	}
	if lb == 0 {
		if la <= limit {
			return la
		}
		return limit + 1
	}
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}
	if lb-la > limit {
		return limit + 1
	}

	// Banded Lowrance-Wagner DP. Only cells with |i-j| <= limit can hold a
	// value <= limit; the rest are treated as infinity. All rows are kept
	// because the transposition term references d[i1-1][j1-1] where i1 is
	// the last row containing the current byte of b.
	const inf = 1 << 30
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		for j := range d[i] {
			d[i][j] = inf
		}
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 0; i <= la; i++ {
		d[i][0] = i
	}
	// da[c] is the 1-based row of the last occurrence of byte c in a.
	var da [256]int

	minInt := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}

	for i := 1; i <= la; i++ {
		db := 0 // 1-based column of the last match in this row
		jlo := i - limit
		if jlo < 1 {
			jlo = 1
		}
		jhi := i + limit
		if jhi > lb {
			jhi = lb
		}
		for j := jlo; j <= jhi; j++ {
			i1 := da[b[j-1]]
			j1 := db
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
				db = j
			}
			best := minInt(d[i-1][j-1]+cost, minInt(d[i][j-1]+1, d[i-1][j]+1))
			if i1 > 0 && j1 > 0 {
				tr := d[i1-1][j1-1] + (i - i1 - 1) + 1 + (j - j1 - 1)
				best = minInt(best, tr)
			}
			// Cells that can only exceed the limit stay at infinity; any
			// path through them yields more than limit edits, so the banded
			// result agrees with the full DP on all values <= limit.
			if best > limit {
				best = inf
			}
			d[i][j] = best
		}
		da[a[i-1]] = i
	}
	if d[la][lb] > limit {
		return limit + 1
	}
	return d[la][lb]
}
