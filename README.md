# go-concurrent-lazy-segment-tree

基于懒惰传播（lazy propagation）线段树的并发安全 Go 库，附带一个演示 CLI。
纯标准库实现，无任何第三方依赖。

## 功能

- `New(values []int64)`：用非空初始值构建线段树
- `RangeAdd(left, right, delta)`：区间 `[left,right)` 整体加 `delta`，O(log n)
- `RangeSum(left, right)` / `RangeMin(left, right)`：区间求和 / 求最小值，O(log n)
- `Get(index)`：单点读取
- `Snapshot()`：返回当前全部值的独立副本
- `Len()`：元素个数（只读）
- `Close()`：关闭后拒绝新的 `RangeAdd`（返回 `ErrClosed`），读操作仍可用，可重复调用

所有区间均为半开区间 `[left, right)`，要求 `0 <= left < right <= Len`；
非法区间或越界下标会返回明确错误。

## 并发模型

写操作（`RangeAdd`）通过 `sync.RWMutex` 互斥执行；读操作
（`RangeSum`/`RangeMin`/`Get`/`Snapshot`）持有读锁并发执行，
查询过程中通过向下携带懒标记（不修改树结构）保证结果正确。

## 目录结构

- `segtree.go` — 线段树库（包名 `lazysegmenttree`）
- `segtree_test.go` — 自动化测试
- `cmd/segtree-cli/main.go` — 演示程序：两次重叠 RangeAdd、区间 Sum/Min、单点读取、Close 后查询

## 运行

```sh
go run ./cmd/segtree-cli        # 运行演示 CLI
go test ./...                   # 运行测试
go test -race ./...             # 带数据竞争检测运行测试
```

## 复杂度

| 操作 | 复杂度 |
| --- | --- |
| 构建 | O(n) |
| RangeAdd / RangeSum / RangeMin | O(log n) |
| Get | O(log n) |
| Snapshot | O(n) |
| 空间 | O(n) |
