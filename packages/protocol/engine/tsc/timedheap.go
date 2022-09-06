package tsc

import (
	"container/heap"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region TimedHeap ////////////////////////////////////////////////////////////////////////////////////////////////////

// TimedHeap defines a heap based on times.
type TimedHeap []*QueueElement

// Len is the number of elements in the collection.
func (h TimedHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i should sort before the element with index j.
func (h TimedHeap) Less(i, j int) bool {
	return h[i].Key.Before(h[j].Key)
}

// Swap swaps the elements with indexes i and j.
func (h TimedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

// Push adds x as the last element to the heap.
func (h *TimedHeap) Push(x interface{}) {
	data := x.(*QueueElement)
	*h = append(*h, data)
	data.index = len(*h) - 1
}

// Pop removes and returns the last element of the heap.
func (h *TimedHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil // avoid memory leak
	*h = (*h)[:n-1]
	data.index = -1
	return data
}

// interface contract (allow the compiler to check if the implementation has all the required methods).
var _ heap.Interface = &TimedHeap{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region QueueElement /////////////////////////////////////////////////////////////////////////////////////////////////

type QueueElement struct {
	// Value represents the value of the queued element.
	Value models.BlockID
	// Key represents the time of the element to be used as a key.
	Key   time.Time
	index int
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
