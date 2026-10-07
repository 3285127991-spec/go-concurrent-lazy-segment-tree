# go-concurrent-lazy-segment-tree

Go 实现的并发安全 Lazy Segment Tree(懒传播线段树),元素类型为 `int64`,仅依赖标准库,无第三方服务。

## 功能

| API | 说明 | 复杂度 |
| --- | --- | --- |
| `New(values []int64) (*Tree, error)` | 由非空初始切片建树 | O(n) |
| `RangeAdd(left, right int, delta int64) error` | 区间 `[left,right)` 整体加 `delta` | O(log n) |
| `RangeSum(left, right int) (int64, error)` | 区间和 | O(log n) |
| `RangeMin(left, right int) (int64, error)` | 区间最小值 | O(log n) |
| `Get(index int) (int64, error)` | 单点读取 | O(log n) |
| `Snapshot() []int64` | 当前全部值的独立副本 | O(n) |
| `Len() int` | 元素个数(只读) | O(1) |
| `Close()` | 关闭写入,幂等 | O(1) |

## 约定

- 所有区间均为半开区间 `[left, right)`,必须满足 `0 <= left < right <= Len`,否则返回明确错误;`index` 越界同样返回错误。
- 初始切片为空时 `New` 返回 `ErrEmptyValues`。
- `Close` 之后:`RangeAdd` 返回 `ErrClosed`;`RangeSum`、`RangeMin`、`Get`、`Snapshot`、`Len` 仍可正常使用;`Close` 可重复调用。

## 实现要点

- 经典懒传播线段树:`sum`/`min`/`lazy` 三个数组,更新在完全覆盖的节点上打懒标记,部分覆盖时下传(`push`)并递归,回溯时重新汇总(`pull`)。
- 并发模型:`sync.RWMutex`。写操作(`RangeAdd`、`Close`)持写锁;读操作持读锁且**不修改树**——查询把祖先的懒标记增量作为参数向下携带(carry),而不是把标记推入子节点,因此多个 reader 之间不会产生数据竞争。
- 负数、重复更新、连续懒标记叠加均由同一套 `apply/push/pull` 不变量保证一致。

## 运行

```powershell
go run ./cmd/segtree-demo   # CLI 演示:两次重叠 RangeAdd、区间 Sum/Min、单点读取、Close 后查询
go test -race ./...         # 全部测试(含竞态检测)
```

## 项目结构

- `segtree.go` — 库实现(包 `lazysegmenttree`)
- `segtree_test.go` — 8 个测试:基础查询、完整覆盖、部分重叠、连续懒更新、负数、边界错误、Snapshot/Close、基础并发
- `cmd/segtree-demo/main.go` — 命令行演示
