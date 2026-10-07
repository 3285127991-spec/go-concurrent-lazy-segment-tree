// Command segtree-cli demonstrates the lazysegmenttree package:
// two overlapping range adds, range sum/min queries, a point read,
// and read queries after Close.
package main

import (
	"fmt"
	"log"

	st "lazysegmenttree"
)

func main() {
	tree, err := st.New([]int64{3, 1, 4, 1, 5, 9, 2, 6})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("initial:", tree.Snapshot(), "len:", tree.Len())

	must := func(err error) {
		if err != nil {
			log.Fatal(err)
		}
	}

	// Two overlapping range adds.
	must(tree.RangeAdd(0, 4, 10))
	fmt.Println("RangeAdd(0, 4, 10)")
	must(tree.RangeAdd(2, 6, -3))
	fmt.Println("RangeAdd(2, 6, -3)")
	fmt.Println("values: ", tree.Snapshot())

	sum, err := tree.RangeSum(0, tree.Len())
	must(err)
	fmt.Println("RangeSum(0, 8) =", sum)

	min, err := tree.RangeMin(2, 6)
	must(err)
	fmt.Println("RangeMin(2, 6) =", min)

	val, err := tree.Get(3)
	must(err)
	fmt.Println("Get(3) =", val)

	// Close blocks new writes but keeps reads available.
	tree.Close()
	tree.Close() // idempotent
	fmt.Println("Close()")

	if err := tree.RangeAdd(0, 1, 1); err != nil {
		fmt.Println("RangeAdd after Close:", err)
	}

	sum, err = tree.RangeSum(0, tree.Len())
	must(err)
	fmt.Println("RangeSum(0, 8) after Close =", sum)
	fmt.Println("Snapshot after Close:  ", tree.Snapshot())
}
