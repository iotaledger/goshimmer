package schedulerutils

import (
	"container/ring"
	"fmt"

	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

const (
	// MaxBufferSize is the maximum total (of all nodes) buffer size in bytes
	MaxBufferSize = 10 * 1024 * 1024
)

// MaxQueueWeight is the maximum mana-scaled inbox size; >= minMessageSize / minAccessMana
var MaxQueueWeight = 1024.0 * 1024

var (
	// ErrInboxExceeded is returned when a node has exceeded its allowed inbox size.
	ErrInboxExceeded = xerrors.New("maximum mana-scaled inbox length exceeded")
	// ErrInvalidMana is returned when the mana is <= 0.
	ErrInvalidMana = xerrors.New("mana cannot be <= 0")
	// ErrBufferFull is returned when the maximum buffer size is exceeded.
	ErrBufferFull = xerrors.New("maximum buffer size exceeded")
)

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of NodeQueue
type BufferQueue struct {
	activeNode map[identity.ID]*ring.Ring
	ring       *ring.Ring
	size       atomic.Uint64
}

// NewBufferQueue returns a new BufferQueue
func NewBufferQueue() *BufferQueue {
	return &BufferQueue{
		activeNode: make(map[identity.ID]*ring.Ring),
		ring:       nil,
	}
}

// NumActiveNodes returns the number of active nodes in b.
func (b *BufferQueue) NumActiveNodes() int {
	return len(b.activeNode)
}

// Size returns the total size (in bytes) of all messages in b.
// Size is thread-safe.
func (b *BufferQueue) Size() uint64 {
	return b.size.Load()
}

// NodeQueue returns the queue for the corresponding node.
func (b *BufferQueue) NodeQueue(nodeID identity.ID) *NodeQueue {
	element, ok := b.activeNode[nodeID]
	if !ok {
		return nil
	}
	return element.Value.(*NodeQueue)
}

// Submit submits a message.
func (b *BufferQueue) Submit(msg Element, rep float64) error {
	if b.Size()+uint64(len(msg.Bytes())) > MaxBufferSize {
		return ErrBufferFull
	}

	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, ok := b.activeNode[nodeID]
	if !ok {
		element = b.ringInsert(NewNodeQueue(nodeID))
		b.activeNode[nodeID] = element
	}

	nodeQueue := element.Value.(*NodeQueue)
	if float64(nodeQueue.Size()+uint(len(msg.Bytes())))/rep > MaxQueueWeight {
		fmt.Println("node queue size: ", nodeQueue.Size())
		fmt.Println("len msg: ", len(msg.Bytes()))
		fmt.Println("rep: ", rep)
		fmt.Println("val: ", float64(nodeQueue.Size()+uint(len(msg.Bytes())))/rep)
		fmt.Println("maxqueue weight: ", MaxQueueWeight)
		return ErrInboxExceeded
	}

	submitted, err := nodeQueue.Submit(msg)
	if err != nil {
		return err
	}
	if !submitted {
		return xerrors.Errorf("error in BufferQueue (Submit): message has already been submitted %x", msg.IDBytes())
	}

	b.size.Add(uint64(len(msg.Bytes())))
	return nil
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (b *BufferQueue) Unsubmit(msg Element) bool {
	nodeID := identity.NewID(msg.IssuerPublicKey())

	element, ok := b.activeNode[nodeID]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*NodeQueue)
	if !nodeQueue.Unsubmit(msg) {
		return false
	}

	b.size.Sub(uint64(len(msg.Bytes())))
	if nodeQueue.IsInactive() {
		b.ringRemove(element)
		delete(b.activeNode, nodeID)
	}
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (b *BufferQueue) Ready(msg Element) bool {
	element, ok := b.activeNode[identity.NewID(msg.IssuerPublicKey())]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*NodeQueue)
	return nodeQueue.Ready(msg)
}

// Next returns the next NodeQueue in round robin order.
func (b *BufferQueue) Next() *NodeQueue {
	if b.ring != nil {
		b.ring = b.ring.Next()
		return b.ring.Value.(*NodeQueue)
	}
	return nil
}

// Current returns the current NodeQueue in round robin order.
func (b *BufferQueue) Current() *NodeQueue {
	if b.ring == nil {
		return nil
	}
	return b.ring.Value.(*NodeQueue)
}

// PopFront removes the first ready message from the queue of the current node.
func (b *BufferQueue) PopFront() Element {
	q := b.Current()
	msg := q.PopFront()
	if q.IsInactive() {
		b.ringRemove(b.ring)
		delete(b.activeNode, identity.NewID(msg.IssuerPublicKey()))
	}

	b.size.Sub(uint64(len(msg.Bytes())))
	return msg
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
