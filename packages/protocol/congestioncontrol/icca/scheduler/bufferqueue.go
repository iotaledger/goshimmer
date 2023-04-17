package scheduler

import (
	"container/ring"
	"math"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
)

// ErrInsufficientMana is returned when the mana is insufficient.
var ErrInsufficientMana = errors.New("insufficient issuer's mana to schedule the block")

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of IssuerQueue.
type BufferQueue struct {
	maxBuffer int

	activeIssuers *shrinkingmap.ShrinkingMap[identity.ID, *ring.Ring]
	ring          *ring.Ring
	size          int
}

// NewBufferQueue returns a new BufferQueue.
func NewBufferQueue(maxBuffer int) *BufferQueue {
	return &BufferQueue{
		maxBuffer:     maxBuffer,
		activeIssuers: shrinkingmap.New[identity.ID, *ring.Ring](),
		ring:          nil,
	}
}

// NumActiveIssuers returns the number of active issuers in b.
func (b *BufferQueue) NumActiveIssuers() int {
	return b.activeIssuers.Size()
}

// MaxSize returns the max number of blocks in BufferQueue.
func (b *BufferQueue) MaxSize() int {
	return b.maxBuffer
}

// Size returns the total number of blocks in BufferQueue.
func (b *BufferQueue) Size() int {
	return b.size
}

// IssuerQueue returns the queue for the corresponding issuer.
func (b *BufferQueue) IssuerQueue(issuerID identity.ID) *IssuerQueue {
	element, ok := b.activeIssuers.Get(issuerID)
	if !ok {
		return nil
	}
	return element.Value.(*IssuerQueue)
}

// Submit submits a block. Return blocks dropped from the scheduler to make room for the submitted block.
// The submitted block can also be returned as dropped if the issuer does not have enough access mana.
func (b *BufferQueue) Submit(blk *Block, accessManaRetriever func(identity.ID) int64) (elements []*Block, err error) {
	issuerID := blk.IssuerID()
	element, issuerActive := b.activeIssuers.Get(issuerID)
	var issuerQueue *IssuerQueue
	if issuerActive {
		issuerQueue = element.Value.(*IssuerQueue)
	} else {
		issuerQueue = NewIssuerQueue(issuerID)
		b.activeIssuers.Set(issuerID, b.ringInsert(issuerQueue))
	}

	// first we submit the block, and if it turns out that the issuer doesn't have enough bandwidth to submit, it will be removed by dropHead
	if !issuerQueue.Submit(blk) {
		return nil, errors.Errorf("block already submitted %s", blk.String())
	}

	b.size++

	// if max buffer size exceeded, drop from head of the longest mana-scaled queue
	if b.Size() > b.maxBuffer {
		return b.dropHead(accessManaRetriever), nil
	}

	return nil, nil
}

func (b *BufferQueue) dropHead(accessManaRetriever func(identity.ID) int64) (droppedBlocks []*Block) {
	start := b.Current()
	// remove as many blocks as necessary to stay within max buffer size
	for b.Size() > b.maxBuffer {
		// TODO: extract to util func
		// find longest mana-scaled queue
		maxScale := math.Inf(-1)
		var maxIssuerID identity.ID
		for q := start; ; {
			issuerMana := accessManaRetriever(q.IssuerID())
			if issuerMana > 0.0 {
				if scale := float64(q.Work()) / float64(issuerMana); scale > maxScale {
					maxScale = scale
					maxIssuerID = q.IssuerID()
				}
			} else if q.Size() > 0 {
				maxScale = math.Inf(1)
				maxIssuerID = q.IssuerID()
			}
			q = b.Next()
			if q == start {
				break
			}
		}
		issuerQueue, _ := b.activeIssuers.Get(maxIssuerID)
		longestQueue := issuerQueue.Value.(*IssuerQueue)

		// TODO: extract to util func
		// find oldest submitted and not-ready block in the longest queue
		var oldestBlock *Block
		longestQueue.submitted.ForEach(func(_ models.BlockID, v *Block) bool {
			if oldestBlock == nil || oldestBlock.IssuingTime().After(v.IssuingTime()) {
				oldestBlock = v
			}
			return true
		})

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
	issuerID := block.IssuerID()

	element, ok := b.activeIssuers.Get(issuerID)
	if !ok {
		return false
	}

	issuerQueue := element.Value.(*IssuerQueue)
	if !issuerQueue.Unsubmit(block) {
		return false
	}

	b.size--
	return true
}

// Ready marks a previously submitted block as ready to be scheduled.
func (b *BufferQueue) Ready(block *Block) bool {
	element, ok := b.activeIssuers.Get(block.IssuerID())
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
		blocksCount += q.submitted.Size()
		q = b.Next()
		if q == start {
			break
		}
	}
	return
}

// InsertIssuer creates a queue for the given issuer and adds it to the list of active issuers.
func (b *BufferQueue) InsertIssuer(issuerID identity.ID) {
	_, issuerActive := b.activeIssuers.Get(issuerID)
	if issuerActive {
		return
	}

	issuerQueue := NewIssuerQueue(issuerID)
	b.activeIssuers.Set(issuerID, b.ringInsert(issuerQueue))
}

// RemoveIssuer removes all blocks (submitted and ready) for the given issuer.
func (b *BufferQueue) RemoveIssuer(issuerID identity.ID) {
	element, ok := b.activeIssuers.Get(issuerID)
	if !ok {
		return
	}

	issuerQueue := element.Value.(*IssuerQueue)
	b.size -= issuerQueue.Size()

	b.ringRemove(element)
	b.activeIssuers.Delete(issuerID)
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

// PopFront removes the first ready block from the queue of the current issuer.
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

// IssuerIDs returns the issuerIDs of all issuers.
func (b *BufferQueue) IssuerIDs() []identity.ID {
	var issuerIDs []identity.ID
	start := b.Current()
	if start == nil {
		return nil
	}
	for q := start; ; {
		issuerIDs = append(issuerIDs, q.IssuerID())
		q = b.Next()
		if q == start {
			break
		}
	}
	return issuerIDs
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
