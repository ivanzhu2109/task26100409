package domaincand

// TLDs 是固定支持的十种顶级域，顺序固定。
var TLDs = []string{"com", "net", "org", "io", "dev", "co", "biz", "info", "app", "xyz"}

// tldIndex 返回 TLD 在 TLDs 中的下标，不存在时返回 -1。
func tldIndex(tld string) int {
	for i, t := range TLDs {
		if t == tld {
			return i
		}
	}
	return -1
}
