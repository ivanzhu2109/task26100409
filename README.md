# domaincand — 本地域名候选检索库

代理采购域名时，针对拼写错误或已被占用的心仪域名，从本地备选词表中
检索最接近的候选。词表与注册占用状态只保存在进程内存中，不依赖任何
外部服务、数据库或监听端口。

## 能力

- `New(labels []string) (*Library, error)`：一次性初始化词表。标签须为
  去重后的合法小写 ASCII 域名标签（`a-z`、`0-9`、`-`，长度 1-63）。
  初始化后词表不再增删，初始全部可用。
- `SetOccupied(domain string, occupied bool) error`：按完整域名
  （如 `example.com`）设置或解除占用。返回后下一次查询立即反映新状态。
- `Query(label, tld string) ([]Candidate, error)`：检索与查询标签编辑
  距离不超过 2 的候选。`tld` 传 `""` 时检索全部十种 TLD，否则只检索
  指定 TLD（如 `"com"`）。

固定支持的 TLD：`.com` `.net` `.org` `.io` `.dev` `.co` `.biz`
`.info` `.app` `.xyz`。每个词表标签与每个 TLD 组合成一个候选域名，
不会生成词表之外的标签。

## 距离定义

将查询标签变成候选标签所需的最少编辑次数：插入、删除、替换一个字符，
或交换两个相邻字符，各计一次；字符允许参与多次编辑（真正的
Damerau-Levenshtein 距离）。例如 `ca` → `abc` 距离为 2（先交换得
`ac`，再插入 `b`）。距离只比较标签，TLD 不计入。

## 结果排序

1. 距离升序；
2. 距离相同时，可用域名排在已占用域名前；
3. 仍相同时，按完整域名（含点号与 TLD）逐 ASCII 字节升序。

返回该全局顺序的前 50 项，不足 50 项全部返回，不重复。排序键构成
全序，结果与词表加载顺序无关。

## 使用示例

```go
lib, err := domaincand.New([]string{"alpha", "alpa", "beta"})
if err != nil { /* 非法或重复标签 */ }

_ = lib.SetOccupied("alpha.com", true)   // 标记占用
_ = lib.SetOccupied("alpha.com", false)  // 解除占用

cands, err := lib.Query("alhpa", "")     // 全部 TLD
cands, err = lib.Query("alhpa", "com")   // 仅 .com
for _, c := range cands {
	fmt.Println(c.Domain, c.Distance, c.Occupied)
}
```

## 实现要点

- 距离计算：先裁剪公共前后缀，再做 Lowrance-Wagner 动态规划，
  行最小值超过 2 即提前退出；与无裁剪参考实现在随机输入上逐一对照。
- 查询过滤：按长度分桶（|Δlen| > 2 直接跳过）+ 字符计数向量
  L1 过滤（距离 d 的计数差不超过 2d），通过者才计算精确距离。
- 占用状态：300 万候选的位图（约 366 KiB），按
  `标签下标×10 + TLD下标` 寻址，变更即时生效。

## 测试

```sh
go test ./...
```

覆盖：相邻交换后继续插入、重复字符、距离恰好 2（入选）与 3（排除）、
超过 50 个同距离候选跨多 TLD 的截断、占用/解除占用后的全局重排、
小词表上与逐一核对全部候选的暴力结果完全一致、词表加载顺序无关性、
非法输入校验。

## 性能验证

```sh
go run ./cmd/perf
```

样例数据：30 万个标签（一半分散随机标签，一半共享长前后缀的密集
标签），标签总长度约 531 万字符，对应 300 万个候选域名；200 个查询
各半覆盖命中与无匹配，其中 1/4 指定单个 TLD。

本机（Apple Silicon, Go 1.27.1）实测：

| 指标 | 实测值 | 要求 |
| --- | --- | --- |
| 建库耗时 | 34 ms | 不计入 |
| 200 个查询总耗时 | 325 ms（平均 1.6 ms/查询） | ≤ 20 s |
| 进程峰值内存 (MaxRSS) | 51.3 MiB | ≤ 512 MiB |
