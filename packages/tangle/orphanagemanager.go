package tangle

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/markers"
)

// region OrphanageManager /////////////////////////////////////////////////////////////////////////////////////////////

// OrphanageManager is a manager that tracks orphaned messages.
type OrphanageManager struct {
	Events *OrphanageManagerEvents

	unconfirmedBlocks   TimedHeap
	tangle              *Tangle
	strongChildCounters map[MessageID]int
	sync.Mutex
}

// NewOrphanageManager returns a new OrphanageManager.
func NewOrphanageManager(tangle *Tangle) *OrphanageManager {
	return &OrphanageManager{
		Events: &OrphanageManagerEvents{
			BlockOrphaned:       event.New[*BlockOrphanedEvent](),
			AllChildrenOrphaned: event.New[*Message](),
		},

		tangle:              tangle,
		strongChildCounters: make(map[MessageID]int),
	}
}

func (o *OrphanageManager) Setup() {
	o.tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *MessageBookedEvent) {
		o.Lock()
		defer o.Unlock()
		o.tangle.Storage.Message(event.MessageID).Consume(func(message *Message) {
			o.addUnconfirmedMessage(message)
		})
	}))

	o.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *MessageConfirmedEvent) {
		o.Lock()
		defer o.Unlock()
		delete(o.strongChildCounters, event.Message.ID())
		o.removeElementFromHeap(event.Message.ID())
	}))

	o.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *TimeUpdate) {
		o.Lock()
		defer o.Unlock()
		o.orphanBeforeTSC(event.ATT.Add(-o.tangle.Options.TimeSinceConfirmationThreshold))
	}))

}

func (o *OrphanageManager) addUnconfirmedMessage(message *Message) {
	heap.Push(&o.unconfirmedBlocks, &QueueElement{Value: message.ID(), Key: message.IssuingTime()})
	for strongParent := range message.ParentsByType(StrongParentType) {
		if strongParent != EmptyMessageID {
			o.strongChildCounters[strongParent]++
		}
	}
}

func (o *OrphanageManager) OrphanBlock(blockID MessageID, reason error) {
	o.Lock()
	defer o.Unlock()
	o.orphanBlockFutureCone(blockID, reason)
}
func (o *OrphanageManager) orphanBlockFutureCone(blockID MessageID, reason error) {
	// TODO: what if marker block is confirmed first, and before confirmation is propagated to its past cone, acceptance time is updated and blocks in the marker's past cone are orphaned?
	// without knowing future markers there is no obvious way to check if the future marker is confirmed
	// a possible solution could be to only rely on markers for orphanage like we do for confirmation. WDYT?

	//if o.isBlockConfirmed(blockID) {
	//	return
	//}

	futureConeWalker := walker.New[MessageID](false).Push(blockID)
	for futureConeWalker.HasNext() {
		blockID = futureConeWalker.Next()
		if reason == nil {
			reason = errors.Errorf("block %s orphaned because its past cone has been orphaned", blockID)
		}

		o.orphanBlock(blockID, reason)
		o.tangle.Storage.Approvers(blockID).Consume(func(approver *Approver) {
			futureConeWalker.Push(approver.ApproverMessageID())
		})
		reason = nil
	}
}

func (o *OrphanageManager) orphanBlock(blockID MessageID, reason error) {
	o.Events.BlockOrphaned.Trigger(&BlockOrphanedEvent{
		BlockID: blockID,
		Reason:  reason,
	})

	// remove blockID from unconfirmed block heap and from childStrongCounters map
	o.removeElementFromHeap(blockID)
	delete(o.strongChildCounters, blockID)

	o.tangle.Storage.Message(blockID).Consume(func(block *Message) {
		for strongParent := range block.ParentsByType(StrongParentType) {
			if strongParent != EmptyMessageID {
				o.decreaseStrongChildCounter(strongParent)
			}
		}
	})
}

func (o *OrphanageManager) decreaseStrongChildCounter(blockID MessageID) {
	if _, exists := o.strongChildCounters[blockID]; !exists {
		return
	}

	o.strongChildCounters[blockID]--

	if o.strongChildCounters[blockID] <= 0 {
		// do not remove block without any approvers from the heap here
		// as it still might get more approvers and be confirmed or orphaned due to TSC threshold
		delete(o.strongChildCounters, blockID)

		o.tangle.Storage.Message(blockID).Consume(func(block *Message) {
			o.Events.AllChildrenOrphaned.Trigger(block)
		})
	}
}

// orphanBeforeTSC removes the elements with key time earlier than the given time.
func (o *OrphanageManager) orphanBeforeTSC(minAllowedTime time.Time) {
	for i := 0; i < o.unconfirmedBlocks.Len(); i++ {
		if o.unconfirmedBlocks[0].Key.After(minAllowedTime) {
			return
		}
		o.orphanBlockFutureCone(o.unconfirmedBlocks[0].Value, errors.Errorf("block %s orphaned as it is older than Time Since Confirmation threshold", o.unconfirmedBlocks[0].Value))
	}
}

// removeElement removes the block from OrphanageManager
func (o *OrphanageManager) removeElementFromHeap(blockID MessageID) {
	for i := 0; i < len(o.unconfirmedBlocks); i++ {
		if o.unconfirmedBlocks[i].Value == blockID {
			heap.Remove(&o.unconfirmedBlocks, o.unconfirmedBlocks[i].index)
			break
		}
	}
}

func (o *OrphanageManager) getNextIndex(sequence *markers.Sequence, markerIndex markers.Index) markers.Index {
	// skip any gaps in marker indices
	markerIndex++
	for ; markerIndex < sequence.HighestIndex(); markerIndex++ {
		currentMarker := markers.NewMarker(sequence.ID(), markerIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if msgID := o.tangle.Booker.MarkersManager.MessageID(currentMarker); msgID == EmptyMessageID {
			continue
		}
		break
	}
	return markerIndex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OrphanageManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

type OrphanageManagerEvents struct {
	BlockOrphaned       *event.Event[*BlockOrphanedEvent]
	AllChildrenOrphaned *event.Event[*Message]
}

type BlockOrphanedEvent struct {
	BlockID MessageID
	Reason  error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimedHeap ////////////////////////////////////////////////////////////////////////////////////////////////////

// TimedHeap defines a heap based on times.
type TimedHeap []*QueueElement

// Len is the number of elements in the collection.
func (h TimedHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i should sort before the element with index j.
func (h TimedHeap) Less(i, j int) bool {
	return h[i].Key.Before(h[j].Key)
}

// Swap swaps the elements with indexes i and j.
func (h TimedHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

// Push adds x as the last element to the heap.
func (h *TimedHeap) Push(x interface{}) {
	data := x.(*QueueElement)
	*h = append(*h, data)
	data.index = len(*h) - 1
}

// Pop removes and returns the last element of the heap.
func (h *TimedHeap) Pop() interface{} {
	n := len(*h)
	data := (*h)[n-1]
	(*h)[n-1] = nil // avoid memory leak
	*h = (*h)[:n-1]
	data.index = -1
	return data
}

// interface contract (allow the compiler to check if the implementation has all the required methods).
var _ heap.Interface = &TimedHeap{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
