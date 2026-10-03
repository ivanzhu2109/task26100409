package domaincand

// maxDistance 是查询返回的最大编辑距离。
const maxDistance = 2

// dlDist2 返回 a 到 b 的 Damerau-Levenshtein 距离（插入、删除、替换、
// 交换两个相邻字符各计一次，字符允许参与多次编辑）；距离超过
// maxDistance 时返回 maxDistance+1。标签最长 63 字符。
func dlDist2(a, b string) int {
	// 去掉公共前缀与公共后缀，不改变编辑距离。
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	a, b = a[p:], b[p:]
	s := 0
	for s < len(a) && s < len(b) && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	a, b = a[:len(a)-s], b[:len(b)-s]

	n, m := len(a), len(b)
	if n == 0 {
		if m > maxDistance {
			return maxDistance + 1
		}
		return m
	}
	if m == 0 {
		if n > maxDistance {
			return maxDistance + 1
		}
		return n
	}
	if n-m > maxDistance || m-n > maxDistance {
		return maxDistance + 1
	}

	// Lowrance-Wagner 动态规划，d[i][j] 为 a[:i] 到 b[:j] 的距离。
	var d [64][64]uint8
	for i := 0; i <= n; i++ {
		d[i][0] = uint8(i)
	}
	for j := 0; j <= m; j++ {
		d[0][j] = uint8(j)
	}
	var da [256]int // 每个字符在 a 中最后出现的行号（1 起始，0 表示未出现）
	for i := 1; i <= n; i++ {
		db := 0 // 当前行中 a[i-1] 在 b 中最后出现的列号
		rowMin := maxDistance + 1
		for j := 1; j <= m; j++ {
			i1 := da[b[j-1]]
			j1 := db
			cost := uint8(1)
			if a[i-1] == b[j-1] {
				cost = 0
				db = j
			}
			v := d[i-1][j] + 1 // 删除
			if x := d[i][j-1] + 1; x < v {
				v = x // 插入
			}
			if x := d[i-1][j-1] + cost; x < v {
				v = x // 替换或匹配
			}
			if i1 > 0 && j1 > 0 {
				// 交换及其中间夹带的删除与插入
				t := int(d[i1-1][j1-1]) + (i - i1 - 1) + 1 + (j - j1 - 1)
				if t < int(v) {
					v = uint8(t)
				}
			}
			d[i][j] = v
			if int(v) < rowMin {
				rowMin = int(v)
			}
		}
		da[a[i-1]] = i
		if rowMin > maxDistance {
			return maxDistance + 1
		}
	}
	if d[n][m] > maxDistance {
		return maxDistance + 1
	}
	return int(d[n][m])
}
