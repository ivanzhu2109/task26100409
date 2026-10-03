# domaincand — 本地域名候选检索库

一个纯进程内存的 Go 库：给定去重后的合法小写 ASCII 域名标签词表，为查询标签找出编辑距离不超过 2 的候选域名，并实时反映各域名的注册占用状态。

## 能力

- **初始化词表**：`New(labels)` 一次性载入，之后不再增删。每个标签与 10 种固定 TLD（`.com .net .org .io .dev .co .biz .info .app .xyz`）组合成候选域名，初始全部可用。
- **变更占用状态**：`SetOccupied(domain, occupied)` 按完整域名设置/解除占用，返回后下一次查询立即反映新状态。
- **查询候选**：`Query(label, tld)` 返回全局有序的前 50 个候选；`tld` 为空串时检索全部 10 种 TLD。

## 使用

```go
idx := domaincand.New([]string{"alpha", "alpa", "beta"})

idx.SetOccupied("alpha.com", true)   // 标记占用
idx.SetOccupied("alpha.com", false)  // 解除占用

for _, c := range idx.Query("alpa", "") {   // 全部 TLD
    fmt.Println(c.Domain, c.Distance, c.Occupied)
}
for _, c := range idx.Query("alpa", "com") { // 指定单个 TLD
    ...
}
```

`Candidate` 字段：`Domain`（完整域名）、`Distance`（标签编辑距离）、`Occupied`（占用状态）。

## 规则说明

- **距离**：完整 Damerau-Levenshtein 距离（Lowrance-Wagner 算法）。插入、删除、替换、相邻交换各计 1 次，字符可参与多次编辑，例如 `ca` → `abc` 距离为 2（先交换得 `ac`，再插入 `b`）。只比较标签，TLD 不计入距离。
- **过滤**：仅返回距离 ≤ 2 的候选；不会生成词表之外的标签。
- **排序**：距离升序 → 可用先于已占用 → 完整域名（含点号与 TLD）逐 ASCII 字节升序；取前 50，不重复。
- **一致性**：排序不依赖词表加载顺序；占用变更后的第一次查询即按新状态重排。

## 实现要点

- 标签组织为一棵前缀 trie（子节点链表，节点 16 字节）；查询时在 trie 上携带 Lowrance-Wagner DP 行做深度优先遍历，行最小值超过 2 即剪枝，共享前缀只计算一次。
- 占用状态是 `len(labels)×10` 的位图（300 万域名仅 ~366 KiB）。
- 排序比较完整域名字节序但不物化字符串（注意 `-` 0x2D 小于 `.` 0x2E，边界逐字节比较）。
- 正确性由差分测试保证：小词表穷举 + 随机模糊词表上，`Query` 结果与逐一核对全部候选的参考实现完全一致；距离函数与 BFS 编辑图预言机在全 `{a,b,c}` 长度 ≤4 字符串上穷举一致。

## 测试

```sh
go test ./...          # 全部测试（含较慢的穷举预言机测试，约 1.5 分钟）
go test -short ./...   # 快速测试
```

## 性能验证

```sh
go run ./cmd/perfcheck
```

样例规模：30 万标签（一半共享长前后缀的密集标签、一半分散随机标签）、总长 5,072,319 字符、300 万候选域名；200 个不同查询（约 1/3 精确命中、1/3 近邻、1/3 无匹配），10% 域名随机置为占用。

本机实测（Apple Silicon，Go 1.27.1）：

| 指标 | 实测 | 预算 |
| --- | --- | --- |
| 建库耗时 | 195 ms | 不计入 |
| 200 次查询总耗时 | 1.27 s（平均 6.4 ms/次） | ≤ 20 s |
| 峰值内存（RSS） | 174.1 MiB | ≤ 512 MiB |
