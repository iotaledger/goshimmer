package scheduler

import (
	"container/heap"
	"fmt"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/generalheap"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/timed"
)

// region IssuerQueue /////////////////////////////////////////////////////////////////////////////////////////////

// IssuerQueue keeps the submitted blocks of an issuer.
type IssuerQueue struct {
	issuerID  identity.ID
	submitted *shrinkingmap.ShrinkingMap[models.BlockID, *Block]
	inbox     generalheap.Heap[timed.HeapKey, *Block]
	size      atomic.Int64
	work      atomic.Int64
}

// NewIssuerQueue returns a new IssuerQueue.
func NewIssuerQueue(issuerID identity.ID) *IssuerQueue {
	return &IssuerQueue{
		issuerID:  issuerID,
		submitted: shrinkingmap.New[models.BlockID, *Block](),
	}
}

// Size returns the total number of blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Size() int {
	if q == nil {
		return 0
	}
	return int(q.size.Load())
}

// Work returns the total work of the blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Work() int {
	if q == nil {
		return 0
	}
	return int(q.work.Load())
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
	q.work.Add(int64(element.Work()))
	return true
}

// Unsubmit removes a previously submitted block from the queue.
func (q *IssuerQueue) Unsubmit(block *Block) bool {
	if _, submitted := q.submitted.Get(block.ID()); !submitted {
		return false
	}

	q.submitted.Delete(block.ID())
	q.size.Dec()
	q.work.Sub(int64(block.Work()))
	return true
}

// Ready marks a previously submitted block as ready to be scheduled.
func (q *IssuerQueue) Ready(block *Block) bool {
	if _, submitted := q.submitted.Get(block.ID()); !submitted {
		return false
	}

	q.submitted.Delete(block.ID())
	heap.Push(&q.inbox, &generalheap.HeapElement[timed.HeapKey, *Block]{Value: block, Key: timed.HeapKey(block.IssuingTime())})
	return true
}

// IDs returns the IDs of all submitted blocks (ready or not).
func (q *IssuerQueue) IDs() (ids []models.BlockID) {
	q.submitted.ForEachKey(func(id models.BlockID) bool {
		ids = append(ids, id)
		return true
	})

	for _, block := range q.inbox {
		ids = append(ids, block.Value.ID())
	}
	return ids
}

// Front returns the first ready block in the queue.
func (q *IssuerQueue) Front() *Block {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return q.inbox[0].Value
}

// PopFront removes the first ready block from the queue.
func (q *IssuerQueue) PopFront() *Block {
	blk := heap.Pop(&q.inbox).(*generalheap.HeapElement[timed.HeapKey, *Block]).Value
	q.size.Dec()
	q.work.Sub(int64(blk.Work()))
	return blk
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
