// Command segtree-demo is a minimal CLI demonstrating the lazysegmenttree
// library: two overlapping range adds, range sum/min queries, a point
// read, and read queries after Close.
package main

import (
	"errors"
	"fmt"
	"log"

	seg "lazysegmenttree"
)

func main() {
	tree, err := seg.New([]int64{5, 2, 8, 1, 9, 3, 7, 4})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("初始值: %v (Len=%d)\n", tree.Snapshot(), tree.Len())

	must := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}

	// 两次重叠的区间加法。
	must(tree.RangeAdd(0, 4, 10))
	fmt.Printf("RangeAdd(0, 4, +10)  -> %v\n", tree.Snapshot())
	must(tree.RangeAdd(2, 6, -3))
	fmt.Printf("RangeAdd(2, 6, -3)   -> %v\n", tree.Snapshot())

	// 区间查询与单点读取。
	sum, err := tree.RangeSum(0, tree.Len())
	must(err)
	min, err := tree.RangeMin(0, tree.Len())
	must(err)
	fmt.Printf("RangeSum(0, 8)=%d  RangeMin(0, 8)=%d\n", sum, min)

	subSum, err := tree.RangeSum(2, 6)
	must(err)
	subMin, err := tree.RangeMin(2, 6)
	must(err)
	fmt.Printf("RangeSum(2, 6)=%d  RangeMin(2, 6)=%d\n", subSum, subMin)

	v, err := tree.Get(3)
	must(err)
	fmt.Printf("Get(3)=%d\n", v)

	// Close 之后:写入被拒绝,读取仍可用,Close 可重复调用。
	tree.Close()
	tree.Close()
	fmt.Println("已 Close (调用两次,幂等)")

	sum, err = tree.RangeSum(0, tree.Len())
	must(err)
	v, err = tree.Get(5)
	must(err)
	fmt.Printf("Close 后: RangeSum(0, 8)=%d  Get(5)=%d  Snapshot=%v\n", sum, v, tree.Snapshot())

	err = tree.RangeAdd(0, 1, 1)
	fmt.Printf("Close 后: RangeAdd(0, 1, 1) 返回错误: %v (errors.Is ErrClosed=%t)\n",
		err, errors.Is(err, seg.ErrClosed))
}
