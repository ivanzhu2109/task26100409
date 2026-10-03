package domaincand

import (
	"fmt"
	"sort"
	"strings"
)

// maxResults 是单次查询返回的最大候选数。
const maxResults = 50

// numCharKinds 是标签字符集（a-z、0-9、-）的大小。
const numCharKinds = 37

// Candidate 是一条查询结果。
type Candidate struct {
	Domain   string // 包含点号与 TLD 的完整域名
	Distance int    // 查询标签与候选标签的编辑距离
	Occupied bool   // 当前是否已被占用
}

// Library 是本地域名候选检索库，词表与占用状态只保存在进程内存中。
// 词表初始化后不再增删；调用按顺序发生，不要求并发安全。
type Library struct {
	labels   []string              // 全部标签
	buckets  [64][]int32           // 按标签长度分桶，值为 labels 下标
	counts   [][numCharKinds]uint8 // 每个标签的字符计数向量
	index    map[string]int32      // 标签 -> labels 下标
	occupied []uint64              // 占用位图，位号 = 标签下标*len(TLDs)+TLD下标
}

// validLabel 校验标签是否为 1 至 63 字符的小写 ASCII 域名标签。
func validLabel(s string) bool {
	if len(s) < 1 || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// charKind 返回标签字符在计数向量中的槽位。
func charKind(c byte) int {
	switch {
	case c >= 'a' && c <= 'z':
		return int(c - 'a')
	case c >= '0' && c <= '9':
		return int(c-'0') + 26
	default: // '-'
		return 36
	}
}

// New 以去重后的合法小写 ASCII 标签集合初始化词表，初始全部可用。
func New(labels []string) (*Library, error) {
	l := &Library{
		labels:   labels,
		counts:   make([][numCharKinds]uint8, len(labels)),
		index:    make(map[string]int32, len(labels)),
		occupied: make([]uint64, (len(labels)*len(TLDs)+63)/64),
	}
	for i, s := range labels {
		if !validLabel(s) {
			return nil, fmt.Errorf("domaincand: 非法标签 %q", s)
		}
		if _, dup := l.index[s]; dup {
			return nil, fmt.Errorf("domaincand: 重复标签 %q", s)
		}
		l.index[s] = int32(i)
		l.buckets[len(s)] = append(l.buckets[len(s)], int32(i))
		for j := 0; j < len(s); j++ {
			l.counts[i][charKind(s[j])]++
		}
	}
	return l, nil
}

// SetOccupied 按完整域名（如 "example.com"）设置或解除占用。
// 域名必须由词表内标签与受支持的 TLD 组成，否则返回错误。
func (l *Library) SetOccupied(domain string, occupied bool) error {
	dot := strings.IndexByte(domain, '.')
	if dot <= 0 {
		return fmt.Errorf("domaincand: 非法域名 %q", domain)
	}
	ti := tldIndex(domain[dot+1:])
	if ti < 0 {
		return fmt.Errorf("domaincand: 不支持的 TLD %q", domain[dot+1:])
	}
	li, ok := l.index[domain[:dot]]
	if !ok {
		return fmt.Errorf("domaincand: 词表外标签 %q", domain[:dot])
	}
	bit := int(li)*len(TLDs) + ti
	if occupied {
		l.occupied[bit/64] |= 1 << (uint(bit) % 64)
	} else {
		l.occupied[bit/64] &^= 1 << (uint(bit) % 64)
	}
	return nil
}

// isOccupied 返回指定候选域名当前是否被占用。
func (l *Library) isOccupied(labelIdx int32, tldIdx int) bool {
	bit := int(labelIdx)*len(TLDs) + tldIdx
	return l.occupied[bit/64]&(1<<(uint(bit)%64)) != 0
}

// Query 检索与 label 距离不超过 2 的候选；tld 为空时检索全部 TLD，
// 否则只检索指定 TLD（如 "com"）。结果按距离升序、可用优先、
// 完整域名逐字节升序排列，最多返回 50 项，不重复。
func (l *Library) Query(label, tld string) ([]Candidate, error) {
	if !validLabel(label) {
		return nil, fmt.Errorf("domaincand: 非法查询标签 %q", label)
	}
	tldFirst, tldLast := 0, len(TLDs)
	if tld != "" {
		ti := tldIndex(tld)
		if ti < 0 {
			return nil, fmt.Errorf("domaincand: 不支持的 TLD %q", tld)
		}
		tldFirst, tldLast = ti, ti+1
	}

	var qc [numCharKinds]uint8
	for i := 0; i < len(label); i++ {
		qc[charKind(label[i])]++
	}

	type match struct {
		idx  int32
		dist int
	}
	var matches []match
	lo := len(label) - maxDistance
	if lo < 1 {
		lo = 1
	}
	hi := len(label) + maxDistance
	if hi > 63 {
		hi = 63
	}
	for length := lo; length <= hi; length++ {
		for _, li := range l.buckets[length] {
			// 字符计数过滤：距离 d 的计数向量 L1 差不超过 2d。
			diff := 0
			for k := 0; k < numCharKinds; k++ {
				d := int(qc[k]) - int(l.counts[li][k])
				if d < 0 {
					d = -d
				}
				diff += d
				if diff > 2*maxDistance {
					break
				}
			}
			if diff > 2*maxDistance {
				continue
			}
			if dist := dlDist2(label, l.labels[li]); dist <= maxDistance {
				matches = append(matches, match{li, dist})
			}
		}
	}

	cands := make([]Candidate, 0, len(matches)*(tldLast-tldFirst))
	for _, mt := range matches {
		base := l.labels[mt.idx]
		for ti := tldFirst; ti < tldLast; ti++ {
			cands = append(cands, Candidate{
				Domain:   base + "." + TLDs[ti],
				Distance: mt.dist,
				Occupied: l.isOccupied(mt.idx, ti),
			})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		if a.Occupied != b.Occupied {
			return !a.Occupied
		}
		return a.Domain < b.Domain
	})
	if len(cands) > maxResults {
		cands = cands[:maxResults]
	}
	return cands, nil
}
