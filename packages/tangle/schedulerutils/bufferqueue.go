package schedulerutils

import (
	"container/ring"
	"math"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
)

// ErrInsufficientMana is returned when the mana is insufficient.
var ErrInsufficientMana = errors.New("insufficient node's mana to schedule the message")

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

// MaxSize returns the max size (in bytes) of all messages in b.
func (b *BufferQueue) MaxSize() int {
	return b.maxBuffer
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

// Submit submits a message. Return messages dropped from the scheduler to make room for the submitted message.
// The submitted message can also be returned as dropped if the issuing node does not have enough access mana.
func (b *BufferQueue) Submit(msg Element, accessManaRetriever func(identity.ID) float64) []ElementID {
	nodeID := identity.NewID(msg.IssuerPublicKey())
	element, nodeActive := b.activeNode[nodeID]
	var nodeQueue *NodeQueue
	if nodeActive {
		nodeQueue = element.Value.(*NodeQueue)
	} else {
		nodeQueue = NewNodeQueue(nodeID)
	}

	// first we submit the message, and if it turns out that the node doesn't have enough bandwidth to submit, it will be removed by dropHead
	if !nodeQueue.Submit(msg) {
		panic("message already submitted")
	}
	// if the node was not active before, add it now
	if !nodeActive {
		b.activeNode[nodeID] = b.ringInsert(nodeQueue)
	}
	b.size++

	// if max buffer size exceeded, drop from head of the longest mana-scaled queue
	if b.Size() > b.maxBuffer {
		return b.dropHead(accessManaRetriever)
	}

	return nil
}

func (b *BufferQueue) dropHead(accessManaRetriever func(identity.ID) float64) (messagesDropped []ElementID) {
	start := b.Current()

	// remove as many messages as necessary to stay within max buffer size
	for b.Size() > b.maxBuffer {
		// find longest mana-scaled queue
		maxScale := math.Inf(-1)
		var maxNodeID identity.ID
		for q := start; ; {
			nodeMana := accessManaRetriever(q.NodeID())
			if nodeMana > 0.0 {
				if scale := float64(q.Size()) / nodeMana; scale > maxScale {
					maxScale = scale
					maxNodeID = q.NodeID()
				}
			} else if q.Size() > 0 {
				maxScale = math.Inf(1)
				maxNodeID = q.NodeID()
			}
			q = b.Next()
			if q == start {
				break
			}
		}
		longestQueue := b.activeNode[maxNodeID].Value.(*NodeQueue)
		// find oldest submitted and not-ready message in the longest queue
		var oldestMessage Element

		for _, v := range longestQueue.submitted {
			if oldestMessage == nil || oldestMessage.IssuingTime().After((*v).IssuingTime()) {
				oldestMessage = *v
			}
		}

		// if the oldest not-ready message is older than the oldest ready message, drop the former otherwise the latter
		readyQueueFront := longestQueue.Front()
		if oldestMessage != nil && (readyQueueFront == nil || oldestMessage.IssuingTime().Before(readyQueueFront.IssuingTime())) {
			messagesDropped = append(messagesDropped, ElementIDFromBytes(oldestMessage.IDBytes()))
			// no need to check if Unsubmit call succeeded, as the mutex of the scheduler is locked to current context
			b.Unsubmit(oldestMessage)
		} else if readyQueueFront != nil {
			msg := longestQueue.PopFront()
			b.size--
			messagesDropped = append(messagesDropped, ElementIDFromBytes(msg.IDBytes()))
		} else {
			panic("scheduler buffer size exceeded and the longest scheduler queue is empty.")
		}
	}
	return messagesDropped
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

	b.size--
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

// ReadyMessagesCount returns the number of ready messages in the buffer.
func (b *BufferQueue) ReadyMessagesCount() (readyMsgCount int) {
	start := b.Current()
	if start == nil {
		return
	}
	for q := start; ; {
		readyMsgCount += q.inbox.Len()
		q = b.Next()
		if q == start {
			break
		}
	}
	return
}

// TotalMessagesCount returns the number of messages in the buffer.
func (b *BufferQueue) TotalMessagesCount() (msgCount int) {
	start := b.Current()
	if start == nil {
		return
	}
	for q := start; ; {
		msgCount += q.inbox.Len()
		msgCount += len(q.submitted)
		q = b.Next()
		if q == start {
			break
		}
	}
	return
}

// InsertNode creates a queue for the given node and adds it to the list of active nodes.
func (b *BufferQueue) InsertNode(nodeID identity.ID) {
	_, nodeActive := b.activeNode[nodeID]
	if nodeActive {
		return
	}

	nodeQueue := NewNodeQueue(nodeID)
	b.activeNode[nodeID] = b.ringInsert(nodeQueue)
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
	b.size--
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
