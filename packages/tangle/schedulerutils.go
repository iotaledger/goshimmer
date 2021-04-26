package tangle

import (
	"container/heap"
	"container/ring"
	"errors"

	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

const (
	// MaxBufferSize is the maximum total (of all nodes) buffer size in bytes
	MaxBufferSize = 10 * 1024 * 1024
)

// ErrBufferFull is returned when the maximum buffer size is exceeded.
var ErrBufferFull = errors.New("maximum buffer size exceeded")

// region NodeQueue /////////////////////////////////////////////////////////////////////////////////////////////

// NodeQueue keeps the submitted messages of a node
type NodeQueue struct {
	nodeID    identity.ID
	submitted map[MessageID]*Message
	inbox     *MessageHeap
	size      uint
}

// NewNodeQueue returns a new NodeQueue
func NewNodeQueue(nodeID identity.ID) *NodeQueue {
	return &NodeQueue{
		nodeID:    nodeID,
		submitted: make(map[MessageID]*Message),
		inbox:     new(MessageHeap),
		size:      0,
	}
}

// IsInactive returns true when the node is inactive, i.e. there are no messages in the queue.
func (q *NodeQueue) IsInactive() bool {
	return q.Size() == 0
}

// Size returns the total size of the messages in the queue.
func (q *NodeQueue) Size() uint {
	if q == nil {
		return 0
	}
	return q.size
}

// NodeID returns the ID of the node belonging to the queue.
func (q *NodeQueue) NodeID() identity.ID {
	return q.nodeID
}

// Submit submits a message for the queue.
func (q *NodeQueue) Submit(msg *Message) (bool, error) {
	msgNodeID := identity.NewID(msg.IssuerPublicKey())
	if q.nodeID != msgNodeID {
		return false, xerrors.Errorf("Queue node ID(%x) and issuer ID(%x) doesn't match.", q.nodeID, msgNodeID)
	}
	if _, submitted := q.submitted[msg.ID()]; submitted {
		return false, nil
	}

	q.submitted[msg.ID()] = msg
	q.size += uint(len(msg.Bytes()))
	return true, nil
}

// Unsubmit removes a previously submitted message from the queue.
func (q *NodeQueue) Unsubmit(msg *Message) bool {
	if _, submitted := q.submitted[msg.ID()]; !submitted {
		return false
	}

	delete(q.submitted, msg.ID())
	q.size -= uint(len(msg.Bytes()))
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (q *NodeQueue) Ready(msg *Message) bool {
	if _, submitted := q.submitted[msg.ID()]; !submitted {
		return false
	}

	delete(q.submitted, msg.ID())
	heap.Push(q.inbox, msg)
	return true
}

// Front returns the first ready message in the queue.
func (q *NodeQueue) Front() *Message {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return (*q.inbox)[0]
}

// PopFront removes the first ready message from the queue.
func (q *NodeQueue) PopFront() *Message {
	msg := heap.Pop(q.inbox).(*Message)
	q.size -= uint(len(msg.Bytes()))
	return msg
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BufferQueue /////////////////////////////////////////////////////////////////////////////////////////////

// BufferQueue represents a buffer of NodeQueue
type BufferQueue struct {
	activeNode map[identity.ID]*ring.Ring
	ring       *ring.Ring
	size       uint
}

// NewBufferQueue returns a new BufferQueue
func NewBufferQueue() *BufferQueue {
	return &BufferQueue{
		activeNode: make(map[identity.ID]*ring.Ring),
		ring:       nil,
		size:       0,
	}
}

// NumActiveNodes returns the number of active nodes in b.
func (b *BufferQueue) NumActiveNodes() int {
	return len(b.activeNode)
}

// Size returns the total size (in bytes) of all messages in b.
func (b *BufferQueue) Size() uint {
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
func (b *BufferQueue) Submit(msg *Message, rep float64) error {
	if b.size+uint(len(msg.Bytes())) > MaxBufferSize {
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
		return ErrInboxExceeded
	}

	submitted, err := nodeQueue.Submit(msg)
	if err != nil {
		return err
	}
	if !submitted {
		return xerrors.Errorf("error in BufferQueue (Submit): message has already been submitted %x", msg.ID)
	}

	b.size += uint(len(msg.Bytes()))
	return nil
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (b *BufferQueue) Unsubmit(msg *Message) bool {
	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, ok := b.activeNode[nodeID]
	if !ok {
		return false
	}

	nodeQueue := element.Value.(*NodeQueue)
	if !nodeQueue.Unsubmit(msg) {
		return false
	}

	b.size -= uint(len(msg.Bytes()))
	if nodeQueue.IsInactive() {
		b.ringRemove(element)
		delete(b.activeNode, nodeID)
	}
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (b *BufferQueue) Ready(msg *Message) bool {
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
func (b *BufferQueue) PopFront() *Message {
	q := b.Current()
	msg := q.PopFront()
	if q.IsInactive() {
		b.ringRemove(b.ring)
		delete(b.activeNode, identity.NewID(msg.IssuerPublicKey()))
	}

	b.size -= uint(len(msg.Bytes()))
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

// region MessageHeap /////////////////////////////////////////////////////////////////////////////////////////////

// MessageHeap holds a heap of messages with respect to their IssuingTime.
type MessageHeap []*Message

// Len is the number of elements in the collection.
func (h MessageHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h MessageHeap) Less(i, j int) bool {
	return h[i].IssuingTime().Before(h[j].IssuingTime())
}

// Swap swaps the elements with indexes i and j.
func (h MessageHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as element with index Len().
// It panics if x is not types.Message.
func (h *MessageHeap) Push(x interface{}) {
	*h = append(*h, x.(*Message))
}

// Pop removes and returns element with index Len() - 1.
func (h *MessageHeap) Pop() interface{} {
	tmp := *h
	n := len(tmp)
	x := tmp[n-1]
	tmp[n-1] = nil
	*h = tmp[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
