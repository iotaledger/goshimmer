package utils

import (
	"container/heap"

	"github.com/iotaledger/hive.go/core/generalheap"
	"github.com/iotaledger/hive.go/core/timed"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the block to be issued) queue size in number of blocks.
	MaxLocalQueueSize = 20
)

// region IssuerQueue /////////////////////////////////////////////////////////////////////////////////////////////

// IssuerQueue keeps the submitted blocks of an issuer.
type IssuerQueue struct {
	inbox generalheap.Heap[timed.HeapKey, *models.Block]
	size  atomic.Int64
	work  atomic.Int64
}

// NewIssuerQueue returns a new IssuerQueue.
func NewIssuerQueue() *IssuerQueue {
	return &IssuerQueue{}
}

// Size returns the total number of blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Size() int {
	if q == nil {
		return 0
	}
	return int(q.size.Load())
}

// Work returns the total bytes of all blocks in the queue.
// This function is thread-safe.
func (q *IssuerQueue) Work() int {
	if q == nil {
		return 0
	}
	return int(q.work.Load())
}

// Enqueue adds block to the queue.
func (q *IssuerQueue) Enqueue(block *models.Block) bool {
	heap.Push(&q.inbox, &generalheap.HeapElement[timed.HeapKey, *models.Block]{Value: block, Key: timed.HeapKey(block.IssuingTime())})
	q.size.Inc()
	q.work.Add(int64(block.Size()))
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
	q.work.Sub(int64(blk.Size()))
	return blk
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
