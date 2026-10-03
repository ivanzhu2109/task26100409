package domaincand

import "sort"

// MaxResults is the maximum number of candidates returned by Query.
const MaxResults = 50

// maxDistance is the largest edit distance included in query results.
const maxDistance = 2

// labelMatch is a vocabulary label within maxDistance of the query.
type labelMatch struct {
	id   uint32
	dist int
}

// Query returns up to MaxResults candidate domains for the given normalized
// label. If tld is empty all supported TLDs are searched; otherwise only the
// given TLD is used. Results are ordered by distance, then availability
// (available before occupied), then full domain name in ASCII byte order.
func (x *Index) Query(label string, tld string) []Candidate {
	tldIDs := make([]int, 0, len(TLDs))
	if tld == "" {
		for i := range TLDs {
			tldIDs = append(tldIDs, i)
		}
	} else {
		id, ok := x.tldID[tld]
		if !ok {
			return nil
		}
		tldIDs = append(tldIDs, id)
	}

	matches := x.searchLabels(label)
	if len(matches) == 0 {
		return nil
	}

	type cand struct {
		labelID uint32
		tldID   int
		dist    int
		occ     bool
	}
	cands := make([]cand, 0, len(matches)*len(tldIDs))
	for _, m := range matches {
		for _, tid := range tldIDs {
			cands = append(cands, cand{
				labelID: m.id,
				tldID:   tid,
				dist:    m.dist,
				occ:     x.occupiedByID(m.id, tid),
			})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		if a.occ != b.occ {
			return !a.occ
		}
		return compareDomains(x.labels[a.labelID], TLDs[a.tldID],
			x.labels[b.labelID], TLDs[b.tldID]) < 0
	})
	if len(cands) > MaxResults {
		cands = cands[:MaxResults]
	}
	out := make([]Candidate, len(cands))
	for i, c := range cands {
		out[i] = Candidate{
			Domain:   x.labels[c.labelID] + "." + TLDs[c.tldID],
			Distance: c.dist,
			Occupied: c.occ,
		}
	}
	return out
}

// compareDomains compares the full domain strings labelA+"."+tldA and
// labelB+"."+tldB byte by byte without materializing them. Labels never
// contain '.', and '-' (0x2D) sorts before '.' (0x2E), so the dot boundary
// must be compared explicitly.
func compareDomains(labelA, tldA, labelB, tldB string) int {
	n := len(labelA)
	if len(labelB) < n {
		n = len(labelB)
	}
	for i := 0; i < n; i++ {
		if labelA[i] != labelB[i] {
			if labelA[i] < labelB[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(labelA) < len(labelB):
		// Next bytes: '.' for A, labelB[n] for B.
		if '.' < labelB[n] {
			return -1
		}
		return 1
	case len(labelA) > len(labelB):
		if labelA[n] < '.' {
			return -1
		}
		return 1
	}
	if tldA < tldB {
		return -1
	}
	if tldA > tldB {
		return 1
	}
	return 0
}

// searchLabels returns every vocabulary label within maxDistance of query,
// using a depth-first walk of the label trie carrying the Lowrance-Wagner
// DP rows along the current path.
func (x *Index) searchLabels(query string) []labelMatch {
	m := len(query)
	// rows[d] is the DP row for the path prefix of depth d.
	rows := make([][]int, x.maxLabelLen+1)
	for d := range rows {
		rows[d] = make([]int, m+1)
	}
	for j := 0; j <= m; j++ {
		rows[0][j] = j
	}
	// da[c] is the 1-based depth of the last occurrence of byte c on the
	// current path; saved and restored around child visits.
	var da [256]int

	var matches []labelMatch
	var walk func(node int32, depth int)
	walk = func(node int32, depth int) {
		i := depth
		prev := rows[i-1]
		cur := rows[i]
		cur[0] = i
		db := 0
		rowMin := cur[0]
		c := x.nodes[node].char
		for j := 1; j <= m; j++ {
			i1 := da[query[j-1]]
			j1 := db
			cost := 1
			if c == query[j-1] {
				cost = 0
				db = j
			}
			best := prev[j-1] + cost
			if v := cur[j-1] + 1; v < best {
				best = v
			}
			if v := prev[j] + 1; v < best {
				best = v
			}
			if i1 > 0 && j1 > 0 {
				if v := rows[i1-1][j1-1] + (i - i1 - 1) + 1 + (j - j1 - 1); v < best {
					best = v
				}
			}
			cur[j] = best
			if best < rowMin {
				rowMin = best
			}
		}
		if rowMin > maxDistance {
			return
		}
		if id := x.nodes[node].labelID; id != noNode && cur[m] <= maxDistance {
			matches = append(matches, labelMatch{id: uint32(id), dist: cur[m]})
		}
		old := da[c]
		da[c] = i
		for child := x.nodes[node].firstChild; child != noNode; child = x.nodes[child].nextSib {
			walk(child, depth+1)
		}
		da[c] = old
	}
	for child := x.nodes[0].firstChild; child != noNode; child = x.nodes[child].nextSib {
		walk(child, 1)
	}
	return matches
}
