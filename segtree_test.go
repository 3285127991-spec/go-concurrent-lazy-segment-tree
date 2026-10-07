package lazysegmenttree_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	seg "lazysegmenttree"
)

// naive is a slice-based reference model used to verify the tree.
type naive struct{ vals []int64 }

func newNaive(values []int64) *naive {
	v := make([]int64, len(values))
	copy(v, values)
	return &naive{vals: v}
}

func (n *naive) add(l, r int, d int64) {
	for i := l; i < r; i++ {
		n.vals[i] += d
	}
}

func (n *naive) sum(l, r int) int64 {
	var s int64
	for i := l; i < r; i++ {
		s += n.vals[i]
	}
	return s
}

func (n *naive) min(l, r int) int64 {
	m := n.vals[l]
	for i := l + 1; i < r; i++ {
		if n.vals[i] < m {
			m = n.vals[i]
		}
	}
	return m
}

// check compares the tree against the naive model on the full range, every
// single index, a few sub-ranges, and Snapshot.
func check(t *testing.T, tree *seg.Tree, n *naive) {
	t.Helper()
	size := len(n.vals)
	ranges := [][2]int{{0, size}}
	for i := 0; i < size; i++ {
		ranges = append(ranges, [2]int{i, i + 1})
	}
	if size >= 4 {
		ranges = append(ranges, [2]int{1, size - 1}, [2]int{0, size / 2}, [2]int{size / 2, size})
	}
	for _, rg := range ranges {
		gotSum, err := tree.RangeSum(rg[0], rg[1])
		if err != nil {
			t.Fatalf("RangeSum(%d, %d) error: %v", rg[0], rg[1], err)
		}
		if want := n.sum(rg[0], rg[1]); gotSum != want {
			t.Errorf("RangeSum(%d, %d) = %d, want %d", rg[0], rg[1], gotSum, want)
		}
		gotMin, err := tree.RangeMin(rg[0], rg[1])
		if err != nil {
			t.Fatalf("RangeMin(%d, %d) error: %v", rg[0], rg[1], err)
		}
		if want := n.min(rg[0], rg[1]); gotMin != want {
			t.Errorf("RangeMin(%d, %d) = %d, want %d", rg[0], rg[1], gotMin, want)
		}
	}
	for i := 0; i < size; i++ {
		got, err := tree.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) error: %v", i, err)
		}
		if got != n.vals[i] {
			t.Errorf("Get(%d) = %d, want %d", i, got, n.vals[i])
		}
	}
	if snap := tree.Snapshot(); !slices.Equal(snap, n.vals) {
		t.Errorf("Snapshot() = %v, want %v", snap, n.vals)
	}
}

func mustNew(t *testing.T, values []int64) *seg.Tree {
	t.Helper()
	tree, err := seg.New(values)
	if err != nil {
		t.Fatalf("New(%v): %v", values, err)
	}
	return tree
}

func mustAdd(t *testing.T, tree *seg.Tree, l, r int, d int64) {
	t.Helper()
	if err := tree.RangeAdd(l, r, d); err != nil {
		t.Fatalf("RangeAdd(%d, %d, %d): %v", l, r, d, err)
	}
}

func TestInitialQueries(t *testing.T) {
	values := []int64{4, -1, 6, 2, 9, 0, 7}
	tree := mustNew(t, values)
	if tree.Len() != len(values) {
		t.Errorf("Len() = %d, want %d", tree.Len(), len(values))
	}
	check(t, tree, newNaive(values))

	// Mutating the input slice after New must not affect the tree.
	values[0] = 100
	if got, _ := tree.Get(0); got != 4 {
		t.Errorf("Get(0) after mutating input slice = %d, want 4", got)
	}
}

func TestFullRangeAdd(t *testing.T) {
	values := []int64{1, 2, 3, 4}
	tree := mustNew(t, values)
	model := newNaive(values)

	mustAdd(t, tree, 0, tree.Len(), 5)
	model.add(0, 4, 5)
	check(t, tree, model)

	mustAdd(t, tree, 0, tree.Len(), -2)
	model.add(0, 4, -2)
	check(t, tree, model)
}

func TestPartialOverlapAdds(t *testing.T) {
	values := []int64{5, 2, 8, 1, 9, 3, 7, 4}
	tree := mustNew(t, values)
	model := newNaive(values)

	mustAdd(t, tree, 0, 4, 10)
	model.add(0, 4, 10)
	mustAdd(t, tree, 2, 6, -3)
	model.add(2, 6, -3)

	check(t, tree, model)
	want := []int64{15, 12, 15, 8, 6, 0, 7, 4}
	if snap := tree.Snapshot(); !slices.Equal(snap, want) {
		t.Errorf("Snapshot() = %v, want %v", snap, want)
	}
}

func TestConsecutiveLazyAdds(t *testing.T) {
	values := []int64{0, 1, 2, 3, 4, 5, 6, 7}
	tree := mustNew(t, values)
	model := newNaive(values)

	ops := []struct {
		l, r int
		d    int64
	}{
		{0, 8, 1}, // lazy mark stays on the root
		{0, 4, 2}, // forces push-down of the root mark
		{4, 8, 3},
		{2, 6, -1}, // partial overlap on both halves
		{3, 5, 5},
		{0, 1, -4},
		{7, 8, 6},
	}
	for _, op := range ops {
		mustAdd(t, tree, op.l, op.r, op.d)
		model.add(op.l, op.r, op.d)
		check(t, tree, model) // verify aggregates after every step
	}
}

func TestNegativeValues(t *testing.T) {
	values := []int64{-5, -2, -8, -1}
	tree := mustNew(t, values)
	model := newNaive(values)

	mustAdd(t, tree, 1, 3, -4)
	model.add(1, 3, -4)
	check(t, tree, model)

	if got, _ := tree.RangeMin(0, 4); got != -12 {
		t.Errorf("RangeMin(0, 4) = %d, want -12", got)
	}
	if got, _ := tree.RangeSum(0, 4); got != -24 {
		t.Errorf("RangeSum(0, 4) = %d, want -24", got)
	}
}

func TestValidation(t *testing.T) {
	if _, err := seg.New(nil); !errors.Is(err, seg.ErrEmptyValues) {
		t.Errorf("New(nil) error = %v, want ErrEmptyValues", err)
	}
	if _, err := seg.New([]int64{}); !errors.Is(err, seg.ErrEmptyValues) {
		t.Errorf("New(empty) error = %v, want ErrEmptyValues", err)
	}

	tree := mustNew(t, []int64{1, 2, 3, 4})
	badRanges := [][2]int{{-1, 2}, {0, 0}, {2, 2}, {3, 2}, {0, 5}, {4, 5}, {-3, -1}}
	for _, rg := range badRanges {
		if err := tree.RangeAdd(rg[0], rg[1], 1); err == nil {
			t.Errorf("RangeAdd(%d, %d, 1) expected error", rg[0], rg[1])
		}
		if _, err := tree.RangeSum(rg[0], rg[1]); err == nil {
			t.Errorf("RangeSum(%d, %d) expected error", rg[0], rg[1])
		}
		if _, err := tree.RangeMin(rg[0], rg[1]); err == nil {
			t.Errorf("RangeMin(%d, %d) expected error", rg[0], rg[1])
		}
	}
	for _, idx := range []int{-1, 4, 100} {
		if _, err := tree.Get(idx); err == nil {
			t.Errorf("Get(%d) expected error", idx)
		}
	}

	// Failed operations must not modify the tree.
	check(t, tree, newNaive([]int64{1, 2, 3, 4}))
}

func TestSnapshotAndClose(t *testing.T) {
	tree := mustNew(t, []int64{1, 2, 3})

	snap := tree.Snapshot()
	snap[0] = 999 // mutating the copy must not affect the tree
	if got, _ := tree.Get(0); got != 1 {
		t.Errorf("Get(0) after mutating snapshot = %d, want 1", got)
	}

	mustAdd(t, tree, 0, 3, 10)
	if got := tree.Snapshot(); !slices.Equal(got, []int64{11, 12, 13}) {
		t.Errorf("Snapshot() = %v, want [11 12 13]", got)
	}
	if !slices.Equal(snap, []int64{999, 2, 3}) {
		t.Errorf("old snapshot changed: %v", snap)
	}

	tree.Close()
	tree.Close() // idempotent

	if err := tree.RangeAdd(0, 1, 1); !errors.Is(err, seg.ErrClosed) {
		t.Errorf("RangeAdd after Close error = %v, want ErrClosed", err)
	}

	// Reads keep working after Close.
	if got, err := tree.RangeSum(0, 3); err != nil || got != 36 {
		t.Errorf("RangeSum after Close = %d, %v; want 36, nil", got, err)
	}
	if got, err := tree.RangeMin(0, 3); err != nil || got != 11 {
		t.Errorf("RangeMin after Close = %d, %v; want 11, nil", got, err)
	}
	if got, err := tree.Get(2); err != nil || got != 13 {
		t.Errorf("Get after Close = %d, %v; want 13, nil", got, err)
	}
	if got := tree.Snapshot(); !slices.Equal(got, []int64{11, 12, 13}) {
		t.Errorf("Snapshot after Close = %v, want [11 12 13]", got)
	}
	if tree.Len() != 3 {
		t.Errorf("Len after Close = %d, want 3", tree.Len())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	values := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	tree := mustNew(t, values)

	const writers = 4
	var wg sync.WaitGroup
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			d := int64(g + 1)
			_ = tree.RangeAdd(0, 4, d)
			_ = tree.RangeAdd(4, 8, d)
			_ = tree.RangeAdd(2, 6, -d)
		}(g)
	}
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < tree.Len(); i++ {
				if _, err := tree.RangeSum(0, tree.Len()); err != nil {
					t.Errorf("RangeSum: %v", err)
					return
				}
				if _, err := tree.RangeMin(0, tree.Len()); err != nil {
					t.Errorf("RangeMin: %v", err)
					return
				}
				if _, err := tree.Get(i); err != nil {
					t.Errorf("Get(%d): %v", i, err)
					return
				}
				if s := tree.Snapshot(); len(s) != tree.Len() {
					t.Errorf("Snapshot len = %d, want %d", len(s), tree.Len())
					return
				}
			}
		}()
	}
	wg.Wait()

	// Adds commute, so the final state is deterministic.
	model := newNaive(values)
	for g := 0; g < writers; g++ {
		d := int64(g + 1)
		model.add(0, 4, d)
		model.add(4, 8, d)
		model.add(2, 6, -d)
	}
	check(t, tree, model)
}
