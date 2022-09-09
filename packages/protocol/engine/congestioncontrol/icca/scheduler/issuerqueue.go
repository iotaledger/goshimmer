package scheduler

import (
	"container/heap"
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region IssuerQueue /////////////////////////////////////////////////////////////////////////////////////////////

// IssuerQueue keeps the submitted blocks of a issuer.
type IssuerQueue struct {
	issuerID  identity.ID
	submitted *shrinkingmap.ShrinkingMap[models.BlockID, *Block]
	inbox     *ElementHeap
	size      atomic.Int64
}

// NewIssuerQueue returns a new IssuerQueue.
func NewIssuerQueue(issuerID identity.ID) *IssuerQueue {
	return &IssuerQueue{
		issuerID:  issuerID,
		submitted: shrinkingmap.New[models.BlockID, *Block](),
		inbox:     new(ElementHeap),
	}
}

// Size returns the total size of the blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Size() int {
	if q == nil {
		return 0
	}
	return int(q.size.Load())
}

// IssuerID returns the ID of the issuer belonging to the queue.
func (q *IssuerQueue) IssuerID() identity.ID {
	return q.issuerID
}

// Submit submits a block for the queue.
func (q *IssuerQueue) Submit(element *Block) bool {
	// this is just a debugging check, it will never happen in practice
	if blkIssuerID := identity.NewID(element.IssuerPublicKey()); q.issuerID != blkIssuerID {
		panic(fmt.Sprintf("issuerqueue: queue issuer ID(%x) and issuer ID(%x) does not match.", q.issuerID, blkIssuerID))
	}

	if _, submitted := q.submitted.Get(element.ID()); submitted {
		return false
	}

	q.submitted.Set(element.ID(), element)
	q.size.Inc()
	return true
}

// Unsubmit removes a previously submitted block from the queue.
func (q *IssuerQueue) Unsubmit(block *Block) bool {
	if _, submitted := q.submitted.Get(block.ID()); !submitted {
		return false
	}

	q.submitted.Delete(block.ID())
	q.size.Dec()
	return true
}

// Ready marks a previously submitted block as ready to be scheduled.
func (q *IssuerQueue) Ready(element *Block) bool {
	if _, submitted := q.submitted.Get(element.ID()); !submitted {
		return false
	}

	q.submitted.Delete(element.ID())
	heap.Push(q.inbox, element)
	return true
}

// IDs returns the IDs of all submitted blocks (ready or not).
func (q *IssuerQueue) IDs() (ids []models.BlockID) {

	q.submitted.ForEachKey(func(id models.BlockID) bool {
		ids = append(ids, id)
		return true
	})

	for _, block := range *q.inbox {
		ids = append(ids, block.ID())
	}
	return ids
}

// Front returns the first ready block in the queue.
func (q *IssuerQueue) Front() *Block {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return (*q.inbox)[0]
}

// PopFront removes the first ready block from the queue.
func (q *IssuerQueue) PopFront() *Block {
	blk := heap.Pop(q.inbox).(*Block)
	q.size.Dec()
	return blk
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ElementHeap /////////////////////////////////////////////////////////////////////////////////////////////

// ElementHeap holds a heap of blocks with respect to their IssuingTime.
type ElementHeap []*Block

// Len is the number of elements in the collection.
func (h ElementHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h ElementHeap) Less(i, j int) bool {
	return h[i].IssuingTime().Before(h[j].IssuingTime())
}

// Swap swaps the elements with indexes i and j.
func (h ElementHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as element with index Len().
// It panics if x is not Element.
func (h *ElementHeap) Push(x any) {
	*h = append(*h, x.(*Block))
}

// Pop removes and returns element with index Len() - 1.
func (h *ElementHeap) Pop() interface{} {
	tmp := *h
	n := len(tmp)
	x := tmp[n-1]
	tmp[n-1] = nil
	*h = tmp[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
