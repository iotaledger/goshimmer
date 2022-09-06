package scheduler

import (
	"container/ring"
	"math"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// ErrInsufficientMana is returned when the mana is insufficient.
var ErrInsufficientMana = errors.New("insufficient node's mana to schedule the block")

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of IssuerQueue.
type BufferQueue struct {
	maxBuffer int

	activeIssuers map[identity.ID]*ring.Ring
	ring          *ring.Ring
	size          int
}

// NewBufferQueue returns a new BufferQueue.
func NewBufferQueue(maxBuffer int) *BufferQueue {
	return &BufferQueue{
		maxBuffer:     maxBuffer,
		activeIssuers: make(map[identity.ID]*ring.Ring),
		ring:          nil,
	}
}

// NumActiveIssuers returns the number of active nodes in b.
func (b *BufferQueue) NumActiveIssuers() int {
	return len(b.activeIssuers)
}

// MaxSize returns the max size (in bytes) of all blocks in b.
func (b *BufferQueue) MaxSize() int {
	return b.maxBuffer
}

// Size returns the total size (in bytes) of all blocks in b.
func (b *BufferQueue) Size() int {
	return b.size
}

// IssuerQueue returns the queue for the corresponding node.
func (b *BufferQueue) IssuerQueue(nodeID identity.ID) *IssuerQueue {
	element, ok := b.activeIssuers[nodeID]
	if !ok {
		return nil
	}
	return element.Value.(*IssuerQueue)
}

// Submit submits a block. Return blocks dropped from the scheduler to make room for the submitted block.
// The submitted block can also be returned as dropped if the issuing node does not have enough access mana.
func (b *BufferQueue) Submit(blk *Block, accessManaRetriever func(identity.ID) float64) (elements []*Block, err error) {
	nodeID := identity.NewID(blk.IssuerPublicKey())
	element, nodeActive := b.activeIssuers[nodeID]
	var nodeQueue *IssuerQueue
	if nodeActive {
		nodeQueue = element.Value.(*IssuerQueue)
	} else {
		nodeQueue = NewIssuerQueue(nodeID)
		b.activeIssuers[nodeID] = b.ringInsert(nodeQueue)
	}

	// first we submit the block, and if it turns out that the node doesn't have enough bandwidth to submit, it will be removed by dropHead
	if !nodeQueue.Submit(blk) {
		return nil, errors.Errorf("block already submitted %s", blk.String())
	}

	b.size++

	// if max buffer size exceeded, drop from head of the longest mana-scaled queue
	if b.Size() > b.maxBuffer {
		return b.dropHead(accessManaRetriever), nil
	}

	return nil, nil
}

func (b *BufferQueue) dropHead(accessManaRetriever func(identity.ID) float64) (droppedBlocks []*Block) {
	start := b.Current()
	// remove as many blocks as necessary to stay within max buffer size
	for b.Size() > b.maxBuffer {
		// TODO: extract to util func
		// find longest mana-scaled queue
		maxScale := math.Inf(-1)
		var maxNodeID identity.ID
		for q := start; ; {
			nodeMana := accessManaRetriever(q.IssuerID())
			if nodeMana > 0.0 {
				if scale := float64(q.Size()) / nodeMana; scale > maxScale {
					maxScale = scale
					maxNodeID = q.IssuerID()
				}
			} else if q.Size() > 0 {
				maxScale = math.Inf(1)
				maxNodeID = q.IssuerID()
			}
			q = b.Next()
			if q == start {
				break
			}
		}
		longestQueue := b.activeIssuers[maxNodeID].Value.(*IssuerQueue)

		// TODO: extract to util func
		// find oldest submitted and not-ready block in the longest queue
		var oldestBlock *Block

		for _, v := range longestQueue.submitted {
			if oldestBlock == nil || oldestBlock.IssuingTime().After(v.IssuingTime()) {
				oldestBlock = v
			}
		}

		// if the oldest not-ready block is older than the oldest ready block, drop the former otherwise the latter
		readyQueueFront := longestQueue.Front()
		if oldestBlock != nil && (readyQueueFront == nil || oldestBlock.IssuingTime().Before(readyQueueFront.IssuingTime())) {
			droppedBlocks = append(droppedBlocks, oldestBlock)

			b.Unsubmit(oldestBlock)
		} else if readyQueueFront != nil {
			blk := longestQueue.PopFront()
			b.size--
			droppedBlocks = append(droppedBlocks, blk)
		} else {
			panic("scheduler buffer size exceeded and the longest scheduler queue is empty.")
		}
	}
	return droppedBlocks
}

// Unsubmit removes a block from the submitted blocks.
// If that block is already marked as ready, Unsubmit has no effect.
func (b *BufferQueue) Unsubmit(block *Block) bool {
	issuerID := identity.NewID(block.IssuerPublicKey())

	element, ok := b.activeIssuers[issuerID]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*IssuerQueue)
	if !nodeQueue.Unsubmit(block) {
		return false
	}

	b.size--
	return true
}

// Ready marks a previously submitted block as ready to be scheduled.
func (b *BufferQueue) Ready(block *Block) bool {
	element, ok := b.activeIssuers[identity.NewID(block.IssuerPublicKey())]
	if !ok {
		return false
	}

	issuerQueue := element.Value.(*IssuerQueue)
	return issuerQueue.Ready(block)
}

// ReadyBlocksCount returns the number of ready blocks in the buffer.
func (b *BufferQueue) ReadyBlocksCount() (readyBlocksCount int) {
	start := b.Current()
	if start == nil {
		return
	}
	for q := start; ; {
		readyBlocksCount += q.inbox.Len()
		q = b.Next()
		if q == start {
			break
		}
	}
	return
}

// TotalBlocksCount returns the number of blocks in the buffer.
func (b *BufferQueue) TotalBlocksCount() (blocksCount int) {
	start := b.Current()
	if start == nil {
		return
	}
	for q := start; ; {
		blocksCount += q.inbox.Len()
		blocksCount += len(q.submitted)
		q = b.Next()
		if q == start {
			break
		}
	}
	return
}

// InsertIssuer creates a queue for the given node and adds it to the list of active nodes.
func (b *BufferQueue) InsertIssuer(nodeID identity.ID) {
	_, nodeActive := b.activeIssuers[nodeID]
	if nodeActive {
		return
	}

	nodeQueue := NewIssuerQueue(nodeID)
	b.activeIssuers[nodeID] = b.ringInsert(nodeQueue)
}

// RemoveIssuer removes all blocks (submitted and ready) for the given node.
func (b *BufferQueue) RemoveIssuer(nodeID identity.ID) {
	element, ok := b.activeIssuers[nodeID]
	if !ok {
		return
	}

	nodeQueue := element.Value.(*IssuerQueue)
	b.size -= nodeQueue.Size()

	b.ringRemove(element)
	delete(b.activeIssuers, nodeID)
}

// Next returns the next IssuerQueue in round-robin order.
func (b *BufferQueue) Next() *IssuerQueue {
	if b.ring != nil {
		b.ring = b.ring.Next()
		return b.ring.Value.(*IssuerQueue)
	}
	return nil
}

// Current returns the current IssuerQueue in round-robin order.
func (b *BufferQueue) Current() *IssuerQueue {
	if b.ring == nil {
		return nil
	}
	return b.ring.Value.(*IssuerQueue)
}

// PopFront removes the first ready block from the queue of the current node.
func (b *BufferQueue) PopFront() (block *Block) {
	q := b.Current()
	block = q.PopFront()
	b.size--
	return block
}

// IDs returns the IDs of all submitted blocks (ready or not).
func (b *BufferQueue) IDs() (ids []models.BlockID) {
	start := b.Current()
	if start == nil {
		return nil
	}
	for q := start; ; {
		ids = append(ids, q.IDs()...)
		q = b.Next()
		if q == start {
			break
		}
	}
	return ids
}

// IssuerIDs returns the nodeIDs of all nodes.
func (b *BufferQueue) IssuerIDs() []identity.ID {
	var nodeIDs []identity.ID
	start := b.Current()
	if start == nil {
		return nil
	}
	for q := start; ; {
		nodeIDs = append(nodeIDs, q.IssuerID())
		q = b.Next()
		if q == start {
			break
		}
	}
	return nodeIDs
}

func (b *BufferQueue) ringRemove(r *ring.Ring) {
	n := b.ring.Next()
	if r == b.ring {
		if n == b.ring {
			b.ring = nil
			return
		}
		b.ring = n
	}
	r.Prev().Link(n)
}

func (b *BufferQueue) ringInsert(v interface{}) *ring.Ring {
	p := ring.New(1)
	p.Value = v
	if b.ring == nil {
		b.ring = p
		return p
	}
	return p.Link(b.ring)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
