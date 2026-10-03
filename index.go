package domaincand

import "strings"

// TLDs is the fixed set of supported top-level domains.
var TLDs = []string{"com", "net", "org", "io", "dev", "co", "biz", "info", "app", "xyz"}

// Candidate is a single query result entry.
type Candidate struct {
	Domain   string
	Distance int
	Occupied bool
}

// trieNode is a node of the label trie. Children are chained through
// nextSib; labelID is -1 for non-terminal nodes. The alphabet is small
// (lower-case letters, digits, hyphen), so sibling chains stay short.
type trieNode struct {
	char       byte
	labelID    int32
	firstChild int32
	nextSib    int32
}

const noNode = -1

// Index holds the label vocabulary, the derived candidate domains and their
// occupancy state. All data lives in process memory.
type Index struct {
	labels      []string
	nodes       []trieNode
	occupied    []uint64 // one bit per (label, TLD) pair
	labelID     map[string]uint32
	tldID       map[string]int
	maxLabelLen int
}

// New builds an index over the given deduplicated, lower-case ASCII labels.
// Every label combined with every supported TLD forms one candidate domain;
// all candidates start out available.
func New(labels []string) *Index {
	x := &Index{
		labels:  labels,
		nodes:   make([]trieNode, 1, 1024),
		labelID: make(map[string]uint32, len(labels)),
		tldID:   make(map[string]int, len(TLDs)),
	}
	x.nodes[0] = trieNode{labelID: noNode, firstChild: noNode, nextSib: noNode}
	for i, tld := range TLDs {
		x.tldID[tld] = i
	}
	for id, label := range labels {
		x.labelID[label] = uint32(id)
		if len(label) > x.maxLabelLen {
			x.maxLabelLen = len(label)
		}
		x.insert(label, int32(id))
	}
	x.occupied = make([]uint64, (len(labels)*len(TLDs)+63)/64)
	return x
}

// insert adds label to the trie and tags its terminal node with id.
func (x *Index) insert(label string, id int32) {
	cur := int32(0)
	for i := 0; i < len(label); i++ {
		c := label[i]
		child := x.nodes[cur].firstChild
		for child != noNode && x.nodes[child].char != c {
			child = x.nodes[child].nextSib
		}
		if child == noNode {
			child = int32(len(x.nodes))
			x.nodes = append(x.nodes, trieNode{
				char:       c,
				labelID:    noNode,
				firstChild: noNode,
				nextSib:    x.nodes[cur].firstChild,
			})
			x.nodes[cur].firstChild = child
		}
		cur = child
	}
	x.nodes[cur].labelID = id
}

// bitIndex returns the occupancy bit position for a (labelID, tldID) pair.
func bitIndex(labelID uint32, tldID int) uint64 {
	return uint64(labelID)*uint64(len(TLDs)) + uint64(tldID)
}

// splitDomain splits a full domain into its label and TLD parts.
func splitDomain(domain string) (label, tld string, ok bool) {
	i := strings.IndexByte(domain, '.')
	if i <= 0 || i == len(domain)-1 {
		return "", "", false
	}
	return domain[:i], domain[i+1:], true
}

// SetOccupied marks the given full domain (label + "." + TLD) as occupied or
// available. It returns false if the domain is not part of the vocabulary.
func (x *Index) SetOccupied(domain string, occupied bool) bool {
	label, tld, ok := splitDomain(domain)
	if !ok {
		return false
	}
	lid, ok := x.labelID[label]
	if !ok {
		return false
	}
	tid, ok := x.tldID[tld]
	if !ok {
		return false
	}
	bit := bitIndex(lid, tid)
	if occupied {
		x.occupied[bit/64] |= 1 << (bit % 64)
	} else {
		x.occupied[bit/64] &^= 1 << (bit % 64)
	}
	return true
}

// Occupied reports the current occupancy state of a full domain. Unknown
// domains report false.
func (x *Index) Occupied(domain string) bool {
	label, tld, ok := splitDomain(domain)
	if !ok {
		return false
	}
	lid, ok := x.labelID[label]
	if !ok {
		return false
	}
	tid, ok := x.tldID[tld]
	if !ok {
		return false
	}
	return x.occupiedByID(lid, tid)
}

func (x *Index) occupiedByID(labelID uint32, tldID int) bool {
	bit := bitIndex(labelID, tldID)
	return x.occupied[bit/64]&(1<<(bit%64)) != 0
}
