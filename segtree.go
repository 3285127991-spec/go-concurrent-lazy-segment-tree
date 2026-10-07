// Package lazysegmenttree implements a concurrency-safe segment tree with
// lazy propagation over int64 values. It supports range add updates and
// range sum / range min queries in O(log n) time.
package lazysegmenttree

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

var (
	// ErrEmptyValues is returned by New when the initial slice is empty.
	ErrEmptyValues = errors.New("lazysegmenttree: initial values must be non-empty")
	// ErrClosed is returned by RangeAdd after Close has been called.
	ErrClosed = errors.New("lazysegmenttree: tree is closed")
)

// Tree is a concurrent lazy segment tree. All ranges are half-open
// intervals [left, right) and must satisfy 0 <= left < right <= Len.
//
// Concurrency model: RangeAdd and Close take the write lock; all read
// operations (RangeSum, RangeMin, Get, Snapshot, Len) take the read lock
// and never mutate the tree. Read-only queries carry pending lazy deltas
// down as a parameter instead of pushing marks into child nodes, so
// concurrent readers cannot race with each other.
type Tree struct {
	mu     sync.RWMutex
	n      int
	sum    []int64 // sum[node] is accurate for the node's segment
	min    []int64 // min[node] is accurate for the node's segment
	lazy   []int64 // lazy[node] is pending for the node's children only
	closed bool
}

// New builds a segment tree from values. values must be non-empty.
func New(values []int64) (*Tree, error) {
	if len(values) == 0 {
		return nil, ErrEmptyValues
	}
	t := &Tree{
		n:    len(values),
		sum:  make([]int64, 4*len(values)),
		min:  make([]int64, 4*len(values)),
		lazy: make([]int64, 4*len(values)),
	}
	t.build(1, 0, t.n, values)
	return t, nil
}

func (t *Tree) build(node, lo, hi int, values []int64) {
	if hi-lo == 1 {
		t.sum[node] = values[lo]
		t.min[node] = values[lo]
		return
	}
	mid := lo + (hi-lo)/2
	t.build(2*node, lo, mid, values)
	t.build(2*node+1, mid, hi, values)
	t.pull(node)
}

// Len returns the number of elements. It is fixed at construction time.
func (t *Tree) Len() int { return t.n }

// RangeAdd adds delta to every element in [left, right).
// It fails with ErrClosed after Close has been called.
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

// RangeSum returns the sum of the elements in [left, right).
func (t *Tree) RangeSum(left, right int) (int64, error) {
	if err := t.checkRange(left, right); err != nil {
		return 0, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.querySum(1, 0, t.n, left, right, 0), nil
}

// RangeMin returns the minimum element in [left, right).
func (t *Tree) RangeMin(left, right int) (int64, error) {
	if err := t.checkRange(left, right); err != nil {
		return 0, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.queryMin(1, 0, t.n, left, right, 0), nil
}

// Get returns the element at index.
func (t *Tree) Get(index int) (int64, error) {
	if index < 0 || index >= t.n {
		return 0, fmt.Errorf("lazysegmenttree: index %d out of bounds [0, %d)", index, t.n)
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	node, lo, hi := 1, 0, t.n
	var carry int64
	for hi-lo > 1 {
		carry += t.lazy[node]
		mid := lo + (hi-lo)/2
		if index < mid {
			node, hi = 2*node, mid
		} else {
			node, lo = 2*node+1, mid
		}
	}
	return t.sum[node] + carry, nil
}

// Snapshot returns an independent copy of all current values.
func (t *Tree) Snapshot() []int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]int64, t.n)
	t.collect(1, 0, t.n, 0, out)
	return out
}

// Close marks the tree as closed: subsequent RangeAdd calls fail with
// ErrClosed, while RangeSum, RangeMin, Get, Snapshot and Len keep working.
// Close is idempotent.
func (t *Tree) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
}

func (t *Tree) checkRange(left, right int) error {
	if left < 0 || right > t.n || left >= right {
		return fmt.Errorf("lazysegmenttree: invalid range [%d, %d) for length %d", left, right, t.n)
	}
	return nil
}

// apply records delta on node: aggregates are updated immediately and the
// delta is queued in lazy[node] for the children.
func (t *Tree) apply(node int, delta int64, segLen int) {
	t.sum[node] += delta * int64(segLen)
	t.min[node] += delta
	t.lazy[node] += delta
}

// push propagates the pending lazy delta of node to its children.
func (t *Tree) push(node, lo, hi int) {
	if t.lazy[node] == 0 {
		return
	}
	mid := lo + (hi-lo)/2
	t.apply(2*node, t.lazy[node], mid-lo)
	t.apply(2*node+1, t.lazy[node], hi-mid)
	t.lazy[node] = 0
}

// pull recomputes node's aggregates from its children.
func (t *Tree) pull(node int) {
	t.sum[node] = t.sum[2*node] + t.sum[2*node+1]
	t.min[node] = min(t.min[2*node], t.min[2*node+1])
}

func (t *Tree) add(node, lo, hi, ql, qr int, delta int64) {
	if qr <= lo || hi <= ql {
		return
	}
	if ql <= lo && hi <= qr {
		t.apply(node, delta, hi-lo)
		return
	}
	t.push(node, lo, hi)
	mid := lo + (hi-lo)/2
	t.add(2*node, lo, mid, ql, qr, delta)
	t.add(2*node+1, mid, hi, ql, qr, delta)
	t.pull(node)
}

// querySum and queryMin are read-only: pending lazy deltas of ancestors are
// carried down in carry instead of being pushed into the children, so
// concurrent readers holding RLock never mutate shared state.

func (t *Tree) querySum(node, lo, hi, ql, qr int, carry int64) int64 {
	if qr <= lo || hi <= ql {
		return 0
	}
	if ql <= lo && hi <= qr {
		return t.sum[node] + carry*int64(hi-lo)
	}
	carry += t.lazy[node]
	mid := lo + (hi-lo)/2
	return t.querySum(2*node, lo, mid, ql, qr, carry) +
		t.querySum(2*node+1, mid, hi, ql, qr, carry)
}

func (t *Tree) queryMin(node, lo, hi, ql, qr int, carry int64) int64 {
	if qr <= lo || hi <= ql {
		return math.MaxInt64
	}
	if ql <= lo && hi <= qr {
		return t.min[node] + carry
	}
	carry += t.lazy[node]
	mid := lo + (hi-lo)/2
	return min(t.queryMin(2*node, lo, mid, ql, qr, carry),
		t.queryMin(2*node+1, mid, hi, ql, qr, carry))
}

func (t *Tree) collect(node, lo, hi int, carry int64, out []int64) {
	if hi-lo == 1 {
		out[lo] = t.sum[node] + carry
		return
	}
	carry += t.lazy[node]
	mid := lo + (hi-lo)/2
	t.collect(2*node, lo, mid, carry, out)
	t.collect(2*node+1, mid, hi, carry, out)
}
