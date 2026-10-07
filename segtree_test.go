package lazysegmenttree

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

func mustNew(t *testing.T, values []int64) *Tree {
	t.Helper()
	tree, err := New(values)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tree
}

func mustSum(t *testing.T, tree *Tree, l, r int) int64 {
	t.Helper()
	sum, err := tree.RangeSum(l, r)
	if err != nil {
		t.Fatalf("RangeSum(%d,%d): %v", l, r, err)
	}
	return sum
}

func mustMin(t *testing.T, tree *Tree, l, r int) int64 {
	t.Helper()
	min, err := tree.RangeMin(l, r)
	if err != nil {
		t.Fatalf("RangeMin(%d,%d): %v", l, r, err)
	}
	return min
}

func mustGet(t *testing.T, tree *Tree, index int) int64 {
	t.Helper()
	v, err := tree.Get(index)
	if err != nil {
		t.Fatalf("Get(%d): %v", index, err)
	}
	return v
}

func naiveSum(values []int64, l, r int) int64 {
	total := int64(0)
	for _, v := range values[l:r] {
		total += v
	}
	return total
}

func naiveMin(values []int64, l, r int) int64 {
	best := values[l]
	for _, v := range values[l:r] {
		if v < best {
			best = v
		}
	}
	return best
}

func TestBasicQueries(t *testing.T) {
	tree := mustNew(t, []int64{3, 1, 4, 1, 5, 9, 2, 6})
	if got := tree.Len(); got != 8 {
		t.Errorf("Len() = %d, want 8", got)
	}
	if got := mustSum(t, tree, 0, 8); got != 31 {
		t.Errorf("RangeSum(0,8) = %d, want 31", got)
	}
	if got := mustSum(t, tree, 2, 5); got != 10 {
		t.Errorf("RangeSum(2,5) = %d, want 10", got)
	}
	if got := mustMin(t, tree, 0, 8); got != 1 {
		t.Errorf("RangeMin(0,8) = %d, want 1", got)
	}
	if got := mustMin(t, tree, 4, 7); got != 2 {
		t.Errorf("RangeMin(4,7) = %d, want 2", got)
	}
	if got := mustGet(t, tree, 0); got != 3 {
		t.Errorf("Get(0) = %d, want 3", got)
	}
	if got := mustGet(t, tree, 7); got != 6 {
		t.Errorf("Get(7) = %d, want 6", got)
	}
}

func TestFullRangeAdd(t *testing.T) {
	tree := mustNew(t, []int64{1, 2, 3, 4})
	if err := tree.RangeAdd(0, 4, 5); err != nil {
		t.Fatalf("RangeAdd: %v", err)
	}
	if got := mustSum(t, tree, 0, 4); got != 30 {
		t.Errorf("RangeSum(0,4) = %d, want 30", got)
	}
	if got := mustMin(t, tree, 0, 4); got != 6 {
		t.Errorf("RangeMin(0,4) = %d, want 6", got)
	}
	if got := mustGet(t, tree, 2); got != 8 {
		t.Errorf("Get(2) = %d, want 8", got)
	}
}

func TestPartialOverlapUpdates(t *testing.T) {
	model := []int64{0, 0, 0, 0, 0, 0, 0, 0}
	tree := mustNew(t, model)
	adds := [][3]int64{{0, 4, 10}, {2, 6, -3}}
	for _, a := range adds {
		if err := tree.RangeAdd(int(a[0]), int(a[1]), a[2]); err != nil {
			t.Fatalf("RangeAdd: %v", err)
		}
		for i := int(a[0]); i < int(a[1]); i++ {
			model[i] += a[2]
		}
	}
	if got := tree.Snapshot(); !slices.Equal(got, model) {
		t.Errorf("Snapshot() = %v, want %v", got, model)
	}
	for l := 0; l < len(model); l++ {
		for r := l + 1; r <= len(model); r++ {
			if got, want := mustSum(t, tree, l, r), naiveSum(model, l, r); got != want {
				t.Errorf("RangeSum(%d,%d) = %d, want %d", l, r, got, want)
			}
			if got, want := mustMin(t, tree, l, r), naiveMin(model, l, r); got != want {
				t.Errorf("RangeMin(%d,%d) = %d, want %d", l, r, got, want)
			}
		}
	}
}

func TestConsecutiveLazyUpdates(t *testing.T) {
	model := make([]int64, 16)
	for i := range model {
		model[i] = int64(i)
	}
	tree := mustNew(t, model)
	adds := [][3]int64{
		{0, 16, 1},
		{3, 9, 5},
		{0, 4, -2},
		{7, 16, 3},
		{5, 6, 10},
		{0, 16, -4},
	}
	for _, a := range adds {
		if err := tree.RangeAdd(int(a[0]), int(a[1]), a[2]); err != nil {
			t.Fatalf("RangeAdd: %v", err)
		}
		for i := int(a[0]); i < int(a[1]); i++ {
			model[i] += a[2]
		}
	}
	if got := tree.Snapshot(); !slices.Equal(got, model) {
		t.Errorf("Snapshot() = %v, want %v", got, model)
	}
	if got, want := mustSum(t, tree, 0, 16), naiveSum(model, 0, 16); got != want {
		t.Errorf("RangeSum(0,16) = %d, want %d", got, want)
	}
	if got, want := mustMin(t, tree, 0, 16), naiveMin(model, 0, 16); got != want {
		t.Errorf("RangeMin(0,16) = %d, want %d", got, want)
	}
	if got, want := mustSum(t, tree, 4, 12), naiveSum(model, 4, 12); got != want {
		t.Errorf("RangeSum(4,12) = %d, want %d", got, want)
	}
}

func TestNegativeValues(t *testing.T) {
	tree := mustNew(t, []int64{-5, -1, -8, 0, -3})
	if err := tree.RangeAdd(1, 4, -7); err != nil {
		t.Fatalf("RangeAdd: %v", err)
	}
	if got := mustSum(t, tree, 0, 5); got != -38 {
		t.Errorf("RangeSum(0,5) = %d, want -38", got)
	}
	if got := mustMin(t, tree, 0, 5); got != -15 {
		t.Errorf("RangeMin(0,5) = %d, want -15", got)
	}
	if got := mustGet(t, tree, 2); got != -15 {
		t.Errorf("Get(2) = %d, want -15", got)
	}
}

func TestInvalidInput(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, ErrEmpty) {
		t.Errorf("New(nil) err = %v, want ErrEmpty", err)
	}
	if _, err := New([]int64{}); !errors.Is(err, ErrEmpty) {
		t.Errorf("New(empty) err = %v, want ErrEmpty", err)
	}
	tree := mustNew(t, []int64{1, 2, 3, 4})
	badRanges := [][2]int{{-1, 2}, {0, 0}, {2, 2}, {3, 2}, {0, 5}, {4, 5}}
	for _, b := range badRanges {
		if err := tree.RangeAdd(b[0], b[1], 1); err == nil {
			t.Errorf("RangeAdd(%d,%d) err = nil, want error", b[0], b[1])
		}
		if _, err := tree.RangeSum(b[0], b[1]); err == nil {
			t.Errorf("RangeSum(%d,%d) err = nil, want error", b[0], b[1])
		}
		if _, err := tree.RangeMin(b[0], b[1]); err == nil {
			t.Errorf("RangeMin(%d,%d) err = nil, want error", b[0], b[1])
		}
	}
	for _, idx := range []int{-1, 4, 100} {
		if _, err := tree.Get(idx); err == nil {
			t.Errorf("Get(%d) err = nil, want error", idx)
		}
	}
	if err := tree.RangeAdd(0, 4, 1); err != nil {
		t.Errorf("RangeAdd(0,4) err = %v, want nil", err)
	}
	if _, err := tree.RangeSum(3, 4); err != nil {
		t.Errorf("RangeSum(3,4) err = %v, want nil", err)
	}
}

func TestSnapshotAndClose(t *testing.T) {
	tree := mustNew(t, []int64{1, 2, 3, 4})
	snap := tree.Snapshot()
	if err := tree.RangeAdd(0, 4, 10); err != nil {
		t.Fatalf("RangeAdd: %v", err)
	}
	if want := []int64{1, 2, 3, 4}; !slices.Equal(snap, want) {
		t.Errorf("snapshot changed after update: %v, want %v", snap, want)
	}
	snap[0] = 999
	if got := mustGet(t, tree, 0); got != 11 {
		t.Errorf("Get(0) = %d after mutating snapshot, want 11", got)
	}

	tree.Close()
	tree.Close() // idempotent
	if err := tree.RangeAdd(0, 1, 1); !errors.Is(err, ErrClosed) {
		t.Errorf("RangeAdd after Close err = %v, want ErrClosed", err)
	}
	if got := mustSum(t, tree, 0, 4); got != 50 {
		t.Errorf("RangeSum(0,4) after Close = %d, want 50", got)
	}
	if got := mustMin(t, tree, 0, 4); got != 11 {
		t.Errorf("RangeMin(0,4) after Close = %d, want 11", got)
	}
	if got := tree.Len(); got != 4 {
		t.Errorf("Len() after Close = %d, want 4", got)
	}
	if got, want := tree.Snapshot(), []int64{11, 12, 13, 14}; !slices.Equal(got, want) {
		t.Errorf("Snapshot() after Close = %v, want %v", got, want)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	const n = 32
	tree := mustNew(t, make([]int64, n))
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if err := tree.RangeAdd(w*4, n-w*4, 1); err != nil {
					t.Errorf("RangeAdd: %v", err)
				}
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if _, err := tree.RangeSum(0, n); err != nil {
					t.Errorf("RangeSum: %v", err)
				}
				if _, err := tree.RangeMin(0, n); err != nil {
					t.Errorf("RangeMin: %v", err)
				}
				if _, err := tree.Get(i % n); err != nil {
					t.Errorf("Get: %v", err)
				}
				_ = tree.Snapshot()
			}
		}()
	}
	wg.Wait()

	expected := make([]int64, n)
	for w := 0; w < 4; w++ {
		for j := w * 4; j < n-w*4; j++ {
			expected[j] += 25
		}
	}
	if got := tree.Snapshot(); !slices.Equal(got, expected) {
		t.Errorf("Snapshot() = %v, want %v", got, expected)
	}
	if got, want := mustSum(t, tree, 0, n), naiveSum(expected, 0, n); got != want {
		t.Errorf("RangeSum(0,%d) = %d, want %d", n, got, want)
	}
}
