package schedulerutils

import (
	"container/heap"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"
)

// region Types ////////////////////////////////////////////////////////////////////////////////////////////////////////

// ElementID is an interface that represents an Entity that can be causally ordered.
type ElementID interface {
	Bytes() []byte
	String() string

	// comparable embeds the comparable interface.
	comparable
}

// Element represents the generic interface for an block in NodeQueue.
type Element interface {
	// IDBytes returns the ID of an Element as a byte slice.
	IDBytes() []byte

	// Size returns the size of the element.
	Size() int

	// IssuerPublicKey returns the issuer public key of the element.
	IssuerPublicKey() ed25519.PublicKey

	// IssuingTime returns the issuing time of the block.
	IssuingTime() time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region NodeQueue /////////////////////////////////////////////////////////////////////////////////////////////

// NodeQueue keeps the submitted blocks of a node.
type NodeQueue[K ElementID, V Element] struct {
	nodeID    identity.ID
	submitted map[K]*V
	inbox     *ElementHeap
	size      atomic.Int64
}

// NewNodeQueue returns a new NodeQueue.
func NewNodeQueue(nodeID identity.ID) *NodeQueue {
	return &NodeQueue{
		nodeID:    nodeID,
		submitted: make(map[ElementID]*Element),
		inbox:     new(ElementHeap),
	}
}

// Size returns the total size of the blocks in the queue.
// This function is thread-safe.
func (q *NodeQueue) Size() int {
	if q == nil {
		return 0
	}
	return int(q.size.Load())
}

// NodeID returns the ID of the node belonging to the queue.
func (q *NodeQueue) NodeID() identity.ID {
	return q.nodeID
}

// Submit submits a block for the queue.
func (q *NodeQueue) Submit(element Element) bool {
	// this is just a debugging check, it will never happen in practice
	if blkNodeID := identity.NewID(element.IssuerPublicKey()); q.nodeID != blkNodeID {
		panic(fmt.Sprintf("nodequeue: queue node ID(%x) and issuer ID(%x) does not match.", q.nodeID, blkNodeID))
	}

	id := ElementIDFromBytes(element.IDBytes())
	if _, submitted := q.submitted[id]; submitted {
		return false
	}

	q.submitted[id] = &element
	q.size.Inc()
	return true
}

// Unsubmit removes a previously submitted block from the queue.
func (q *NodeQueue) Unsubmit(element Element) bool {
	id := ElementIDFromBytes(element.IDBytes())
	if _, submitted := q.submitted[id]; !submitted {
		return false
	}

	delete(q.submitted, id)
	q.size.Dec()
	return true
}

// Ready marks a previously submitted block as ready to be scheduled.
func (q *NodeQueue) Ready(element Element) bool {
	id := ElementIDFromBytes(element.IDBytes())
	if _, submitted := q.submitted[id]; !submitted {
		return false
	}

	delete(q.submitted, id)
	heap.Push(q.inbox, element)
	return true
}

// IDs returns the IDs of all submitted blocks (ready or not).
func (q *NodeQueue) IDs() (ids []ElementID) {
	for id := range q.submitted {
		ids = append(ids, id)
	}
	for _, element := range *q.inbox {
		ids = append(ids, ElementIDFromBytes(element.IDBytes()))
	}
	return ids
}

// Front returns the first ready block in the queue.
func (q *NodeQueue) Front() Element {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return (*q.inbox)[0]
}

// PopFront removes the first ready block from the queue.
func (q *NodeQueue) PopFront() Element {
	blk := heap.Pop(q.inbox).(Element)
	q.size.Dec()
	return blk
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ElementHeap /////////////////////////////////////////////////////////////////////////////////////////////

// ElementHeap holds a heap of blocks with respect to their IssuingTime.
type ElementHeap[V Element] []V

// Len is the number of elements in the collection.
func (h ElementHeap[V]) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h ElementHeap[V]) Less(i, j int) bool {
	return h[i].IssuingTime().Before(h[j].IssuingTime())
}

// Swap swaps the elements with indexes i and j.
func (h ElementHeap[V]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as element with index Len().
// It panics if x is not Element.
func (h *ElementHeap[V]) Push(x V) {
	*h = append(*h, x)
}

// Pop removes and returns element with index Len() - 1.
func (h *ElementHeap[V]) Pop() interface{} {
	tmp := *h
	n := len(tmp)
	x := tmp[n-1]
	tmp[n-1] = nil
	*h = tmp[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
