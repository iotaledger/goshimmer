package ratesetter

import (
	"container/heap"

	"github.com/iotaledger/hive.go/core/generalheap"
	"github.com/iotaledger/hive.go/core/timed"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region IssuerQueue /////////////////////////////////////////////////////////////////////////////////////////////

// IssuerQueue keeps the submitted blocks of an issuer.
type IssuerQueue struct {
	inbox generalheap.Heap[timed.HeapKey, *models.Block]
	size  atomic.Int64
}

// NewIssuerQueue returns a new IssuerQueue.
func NewIssuerQueue() *IssuerQueue {
	return &IssuerQueue{}
}

// Size returns the total size of the blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Size() int {
	if q == nil {
		return 0
	}
	return int(q.size.Load())
}

// Enqueue adds block to the queue.
func (q *IssuerQueue) Enqueue(block *models.Block) bool {
	heap.Push(&q.inbox, &generalheap.HeapElement[timed.HeapKey, *models.Block]{Value: block, Key: timed.HeapKey(block.IssuingTime())})
	return true
}

// IDs returns the IDs of all submitted blocks (ready or not).
func (q *IssuerQueue) IDs() (ids []models.BlockID) {
	for _, block := range q.inbox {
		ids = append(ids, block.Value.ID())
	}
	return ids
}

// Front returns the first ready block in the queue.
func (q *IssuerQueue) Front() *models.Block {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return q.inbox[0].Value
}

// PopFront removes the first ready block from the queue.
func (q *IssuerQueue) PopFront() *models.Block {
	blk := heap.Pop(&q.inbox).(*generalheap.HeapElement[timed.HeapKey, *models.Block]).Value
	q.size.Dec()
	return blk
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
