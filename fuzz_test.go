package domaincand

import (
	"math/rand"
	"testing"
)

// TestQueryFuzzDifferential hammers the trie search with random vocabularies
// over a small alphabet (forcing many matches, repeated characters and
// transposition opportunities) and compares against the brute-force
// reference on every query.
func TestQueryFuzzDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const alphabet = "abcd"
	randString := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(b)
	}
	for trial := 0; trial < 40; trial++ {
		seen := map[string]bool{}
		var labels []string
		for len(labels) < 120 {
			s := randString(1 + rng.Intn(8))
			if !seen[s] {
				seen[s] = true
				labels = append(labels, s)
			}
		}
		x := New(labels)
		for _, l := range labels {
			for _, tld := range TLDs {
				if rng.Float64() < 0.25 {
					x.SetOccupied(l+"."+tld, true)
				}
			}
		}
		for qi := 0; qi < 15; qi++ {
			q := randString(1 + rng.Intn(8))
			tld := ""
			if qi%4 == 0 {
				tld = TLDs[rng.Intn(len(TLDs))]
			}
			checkQuery(t, x, q, tld)
		}
	}
}
