package schedulerutils

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

// ElementIDLength defines the length of an ElementID.
const ElementIDLength = 32

// ElementID defines the ID of element.
type ElementID [ElementIDLength]byte

// ElementIDFromBytes converts byte array to an ElementID.
func ElementIDFromBytes(bytes []byte) (result ElementID, err error) {
	// check arguments
	if len(bytes) < ElementIDLength {
		err = fmt.Errorf("bytes not long enough to encode a valid message id")
		return
	}

	copy(result[:], bytes)
	return
}

// Element represents the generic interface for an message in NodeQueue.
type Element interface {
	// IDBytes returns the ID of an Element as a byte slice.
	IDBytes() []byte

	// Bytes returns a marshaled version of the Element.
	Bytes() []byte

	// IssuerPublicKey returns the issuer public key of the element.
	IssuerPublicKey() ed25519.PublicKey

	// IssuingTime returns the issuing time of the message.
	IssuingTime() time.Time
}

// region NodeQueue /////////////////////////////////////////////////////////////////////////////////////////////

// NodeQueue keeps the submitted messages of a node
type NodeQueue struct {
	nodeID    identity.ID
	submitted map[ElementID]*Element
	inbox     *ElementHeap
	size      uint

	submittedMutex sync.Mutex
}

// NewNodeQueue returns a new NodeQueue
func NewNodeQueue(nodeID identity.ID) *NodeQueue {
	return &NodeQueue{
		nodeID:    nodeID,
		submitted: make(map[ElementID]*Element),
		inbox:     new(ElementHeap),
		size:      0,
	}
}

// IsInactive returns true when the node is inactive, i.e. there are no messages in the queue.
func (q *NodeQueue) IsInactive() bool {
	return q.Size() == 0
}

// SetSize sets the size of NodeQueue (for testing).
func (q *NodeQueue) SetSize(size uint) {
	q.size = size
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
func (q *NodeQueue) Submit(element Element) (bool, error) {
	msgNodeID := identity.NewID(element.IssuerPublicKey())
	if q.nodeID != msgNodeID {
		return false, xerrors.Errorf("Queue node ID(%x) and issuer ID(%x) doesn't match.", q.nodeID, msgNodeID)
	}

	q.submittedMutex.Lock()
	defer q.submittedMutex.Unlock()
	id, err := ElementIDFromBytes(element.IDBytes())
	if err != nil {
		return false, err
	}
	if _, submitted := q.submitted[id]; submitted {
		return false, nil
	}

	q.submitted[id] = &element
	q.size += uint(len(element.Bytes()))
	return true, nil
}

// Unsubmit removes a previously submitted message from the queue.
func (q *NodeQueue) Unsubmit(element Element) bool {
	q.submittedMutex.Lock()
	defer q.submittedMutex.Unlock()
	id, err := ElementIDFromBytes(element.IDBytes())
	if err != nil {
		return false
	}
	if _, submitted := q.submitted[id]; !submitted {
		return false
	}

	delete(q.submitted, id)
	q.size -= uint(len(element.Bytes()))
	return true
}

// Ready marks a previously submitted message as ready to be scheduled.
func (q *NodeQueue) Ready(element Element) bool {
	q.submittedMutex.Lock()
	defer q.submittedMutex.Unlock()
	id, err := ElementIDFromBytes(element.IDBytes())
	if err != nil {
		return false
	}
	if _, submitted := q.submitted[id]; !submitted {
		return false
	}

	delete(q.submitted, id)
	heap.Push(q.inbox, element)
	return true
}

// Front returns the first ready message in the queue.
func (q *NodeQueue) Front() Element {
	if q == nil || q.inbox.Len() == 0 {
		return nil
	}
	return (*q.inbox)[0]
}

// PopFront removes the first ready message from the queue.
func (q *NodeQueue) PopFront() Element {
	msg := heap.Pop(q.inbox).(Element)
	q.size -= uint(len(msg.Bytes()))
	return msg
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ElementHeap /////////////////////////////////////////////////////////////////////////////////////////////

// ElementHeap holds a heap of messages with respect to their IssuingTime.
type ElementHeap []Element

// Len is the number of elements in the collection.
func (h ElementHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h ElementHeap) Less(i, j int) bool {
	return h[i].IssuingTime().Before(h[j].IssuingTime())
}

// Swap swaps the elements with indexes i and j.
func (h ElementHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as element with index Len().
// It panics if x is not Element.
func (h *ElementHeap) Push(x interface{}) {
	*h = append(*h, x.(Element))
}

// Pop removes and returns element with index Len() - 1.
func (h *ElementHeap) Pop() interface{} {
	tmp := *h
	n := len(tmp)
	x := tmp[n-1]
	tmp[n-1] = nil
	*h = tmp[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
