// Package lazysegmenttree provides a concurrency-safe segment tree with
// lazy propagation supporting range add, range sum and range min queries
// over a fixed []int64. All ranges are half-open: [left, right).
package lazysegmenttree

import (
	"errors"
	"fmt"
	"sync"
)

// ErrEmpty is returned by New when the initial value slice is empty.
var ErrEmpty = errors.New("lazysegmenttree: initial values must not be empty")

// ErrClosed is returned by RangeAdd after the tree has been closed.
var ErrClosed = errors.New("lazysegmenttree: tree is closed")

// node is one segment tree node. lazy is a pending range-add that has been
// applied to this node's aggregates but not yet pushed to its children.
type node struct {
	sum  int64
	min  int64
	lazy int64
}

// Tree is a lazy-propagation segment tree over a fixed number of int64
// values. It is safe for concurrent use: writers are serialized and
// readers run concurrently via a sync.RWMutex.
type Tree struct {
	mu     sync.RWMutex
	n      int
	nodes  []node
	closed bool
}

// New builds a segment tree from the given initial values. The slice must
// be non-empty; its contents are copied into the tree.
func New(values []int64) (*Tree, error) {
	if len(values) == 0 {
		return nil, ErrEmpty
	}
	t := &Tree{
		n:     len(values),
		nodes: make([]node, 4*len(values)),
	}
	t.build(1, 0, t.n, values)
	return t, nil
}

func (t *Tree) build(i, l, r int, values []int64) {
	if l+1 == r {
		t.nodes[i].sum = values[l]
		t.nodes[i].min = values[l]
		return
	}
	mid := (l + r) / 2
	t.build(2*i, l, mid, values)
	t.build(2*i+1, mid, r, values)
	t.pull(i)
}

// pull recomputes node i's aggregates from its children.
func (t *Tree) pull(i int) {
	t.nodes[i].sum = t.nodes[2*i].sum + t.nodes[2*i+1].sum
	t.nodes[i].min = min(t.nodes[2*i].min, t.nodes[2*i+1].min)
}

// apply adds delta to every element covered by node i (range [l, r)) and
// records it in the node's lazy tag.
func (t *Tree) apply(i, l, r int, delta int64) {
	t.nodes[i].sum += delta * int64(r-l)
	t.nodes[i].min += delta
	t.nodes[i].lazy += delta
}

// push propagates node i's lazy tag to its children.
func (t *Tree) push(i, l, r int) {
	if t.nodes[i].lazy == 0 || l+1 == r {
		return
	}
	mid := (l + r) / 2
	t.apply(2*i, l, mid, t.nodes[i].lazy)
	t.apply(2*i+1, mid, r, t.nodes[i].lazy)
	t.nodes[i].lazy = 0
}

func (t *Tree) checkRange(left, right int) error {
	if left < 0 || left >= right || right > t.n {
		return fmt.Errorf("lazysegmenttree: invalid range [%d,%d) for length %d", left, right, t.n)
	}
	return nil
}

// RangeAdd adds delta to every element in [left, right).
// It fails with ErrClosed once the tree has been closed.
func (t *Tree) RangeAdd(left, right int, delta int64) error {
	if err := t.checkRange(left, right); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return ErrClosed
	}
	t.add(1, 0, t.n, left, right, delta)
	return nil
}

func (t *Tree) add(i, l, r, ql, qr int, delta int64) {
	if ql <= l && r <= qr {
		t.apply(i, l, r, delta)
		return
	}
	t.push(i, l, r)
	mid := (l + r) / 2
	if ql < mid {
		t.add(2*i, l, mid, ql, qr, delta)
	}
	if mid < qr {
		t.add(2*i+1, mid, r, ql, qr, delta)
	}
	t.pull(i)
}

// RangeSum returns the sum of the elements in [left, right).
func (t *Tree) RangeSum(left, right int) (int64, error) {
	if err := t.checkRange(left, right); err != nil {
		return 0, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sum(1, 0, t.n, left, right, 0), nil
}

// carry holds lazy tags of ancestors that have not been pushed down, so
// queries never mutate the tree and can run under a read lock.
func (t *Tree) sum(i, l, r, ql, qr int, carry int64) int64 {
	if ql <= l && r <= qr {
		return t.nodes[i].sum + carry*int64(r-l)
	}
	mid := (l + r) / 2
	carry += t.nodes[i].lazy
	total := int64(0)
	if ql < mid {
		total += t.sum(2*i, l, mid, ql, qr, carry)
	}
	if mid < qr {
		total += t.sum(2*i+1, mid, r, ql, qr, carry)
	}
	return total
}

// RangeMin returns the minimum of the elements in [left, right).
func (t *Tree) RangeMin(left, right int) (int64, error) {
	if err := t.checkRange(left, right); err != nil {
		return 0, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.minAt(1, 0, t.n, left, right, 0), nil
}

func (t *Tree) minAt(i, l, r, ql, qr int, carry int64) int64 {
	if ql <= l && r <= qr {
		return t.nodes[i].min + carry
	}
	mid := (l + r) / 2
	carry += t.nodes[i].lazy
	best := int64(0)
	if ql < mid {
		best = t.minAt(2*i, l, mid, ql, qr, carry)
	} else {
		best = t.minAt(2*i+1, mid, r, ql, qr, carry)
	}
	if ql < mid && mid < qr {
		if right := t.minAt(2*i+1, mid, r, ql, qr, carry); right < best {
			best = right
		}
	}
	return best
}

// Get returns the value at the given index.
func (t *Tree) Get(index int) (int64, error) {
	if index < 0 || index >= t.n {
		return 0, fmt.Errorf("lazysegmenttree: index %d out of bounds [0,%d)", index, t.n)
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.get(1, 0, t.n, index, 0), nil
}

func (t *Tree) get(i, l, r, index int, carry int64) int64 {
	if l+1 == r {
		return t.nodes[i].sum + carry
	}
	mid := (l + r) / 2
	carry += t.nodes[i].lazy
	if index < mid {
		return t.get(2*i, l, mid, index, carry)
	}
	return t.get(2*i+1, mid, r, index, carry)
}

// Snapshot returns an independent copy of all current values.
func (t *Tree) Snapshot() []int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]int64, t.n)
	t.collect(1, 0, t.n, 0, out)
	return out
}

func (t *Tree) collect(i, l, r int, carry int64, out []int64) {
	if l+1 == r {
		out[l] = t.nodes[i].sum + carry
		return
	}
	mid := (l + r) / 2
	carry += t.nodes[i].lazy
	t.collect(2*i, l, mid, carry, out)
	t.collect(2*i+1, mid, r, carry, out)
}

// Len returns the number of elements in the tree. It is immutable.
func (t *Tree) Len() int {
	return t.n
}

// Close marks the tree closed: subsequent RangeAdd calls fail with
// ErrClosed, while all read operations keep working. Close is idempotent.
func (t *Tree) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
}
