package schedulerutils

import (
	"container/ring"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
)

var (
	// ErrInboxExceeded is returned when a node has exceeded its allowed inbox size.
	ErrInboxExceeded = errors.New("maximum mana-scaled inbox length exceeded")
	// ErrInsufficientMana is returned when the mana is insufficient.
	ErrInsufficientMana = errors.New("insufficient node's mana to schedule the message")
	// ErrBufferFull is returned when the maximum buffer size is exceeded.
	ErrBufferFull = errors.New("maximum buffer size exceeded")
)

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of NodeQueue.
type BufferQueue struct {
	maxBuffer int
	maxQueue  float64

	activeNode map[identity.ID]*ring.Ring
	ring       *ring.Ring
	size       int
}

// NewBufferQueue returns a new BufferQueue.
func NewBufferQueue(maxBuffer int, maxQueue float64) *BufferQueue {
	return &BufferQueue{
		maxBuffer:  maxBuffer,
		maxQueue:   maxQueue,
		activeNode: make(map[identity.ID]*ring.Ring),
		ring:       nil,
	}
}

// NumActiveNodes returns the number of active nodes in b.
func (b *BufferQueue) NumActiveNodes() int {
	return len(b.activeNode)
}

// Size returns the total size (in bytes) of all messages in b.
func (b *BufferQueue) Size() int {
	return b.size
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
	size := msg.Size()
	if b.size+size > b.maxBuffer {
		return ErrBufferFull
	}

	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, nodeActive := b.activeNode[nodeID]
	var nodeQueue *NodeQueue
	if nodeActive {
		nodeQueue = element.Value.(*NodeQueue)
	} else {
		nodeQueue = NewNodeQueue(nodeID)
	}

	if float64(nodeQueue.Size()+size)/rep > b.maxQueue {
		return ErrInboxExceeded
	}
	if !nodeQueue.Submit(msg) {
		panic("message already submitted")
	}

	// if the node was not active before, add it now
	if !nodeActive {
		b.activeNode[nodeID] = b.ringInsert(nodeQueue)
	}
	b.size += size
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

	b.size -= msg.Size()
	if nodeQueue.Size() == 0 {
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

// RemoveNode removes all messages (submitted and ready) for the given node.
func (b *BufferQueue) RemoveNode(nodeID identity.ID) {
	element, ok := b.activeNode[nodeID]
	if !ok {
		return
	}

	nodeQueue := element.Value.(*NodeQueue)
	b.size -= nodeQueue.Size()

	b.ringRemove(element)
	delete(b.activeNode, nodeID)
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
	if q.Size() == 0 {
		b.ringRemove(b.ring)
		delete(b.activeNode, identity.NewID(msg.IssuerPublicKey()))
	}

	b.size -= msg.Size()
	return msg
}

// IDs returns the IDs of all submitted messages (ready or not).
func (b *BufferQueue) IDs() (ids []ElementID) {
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

// NodeIDs returns the nodeIDs of all nodes.
func (b *BufferQueue) NodeIDs() []identity.ID {
	var nodeIDs []identity.ID
	start := b.Current()
	if start == nil {
		return nil
	}
	for q := start; ; {
		nodeIDs = append(nodeIDs, q.NodeID())
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
