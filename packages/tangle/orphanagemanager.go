package tangle

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/pkg/errors"
)

// region OrphanageManager /////////////////////////////////////////////////////////////////////////////////////////////

// OrphanageManager is a manager that tracks orphaned blocks.
type OrphanageManager struct {
	Events *OrphanageManagerEvents

	unconfirmedBlocks   TimedHeap
	tangle              *Tangle
	strongChildCounters map[BlockID]int
	sync.Mutex
}

// NewOrphanageManager returns a new OrphanageManager.
func NewOrphanageManager(tangle *Tangle) *OrphanageManager {
	return &OrphanageManager{
		Events: &OrphanageManagerEvents{
			BlockOrphaned:       event.New[*BlockOrphanedEvent](),
			AllChildrenOrphaned: event.New[*Block](),
		},

		tangle:              tangle,
		strongChildCounters: make(map[BlockID]int),
	}
}

func (o *OrphanageManager) Setup() {
	o.tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *BlockBookedEvent) {
		o.Lock()
		defer o.Unlock()
		o.tangle.Storage.Block(event.BlockID).Consume(func(block *Block) {
			o.addUnconfirmedBlock(block)
		})
	}))

	// Handle this event synchronously to guarantee that confirmed block is removed from orphanage manager before
	// acceptance time is updated for this block as this could lead to some inconsistencies and manager trying to
	// orphan confirmed messages.
	o.tangle.ConfirmationOracle.Events().BlockAccepted.Hook(event.NewClosure(func(event *BlockAcceptedEvent) {
		o.Lock()
		defer o.Unlock()
		delete(o.strongChildCounters, event.Block.ID())
		o.removeElementFromHeap(event.Block.ID())
	}))

	o.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *TimeUpdate) {
		o.Lock()
		defer o.Unlock()
		o.orphanBeforeTSC(event.ATT.Add(-o.tangle.Options.TimeSinceConfirmationThreshold))
	}))

}

func (o *OrphanageManager) addUnconfirmedBlock(block *Block) {
	heap.Push(&o.unconfirmedBlocks, &QueueElement{Value: block.ID(), Key: block.IssuingTime()})
	for strongParent := range block.ParentsByType(StrongParentType) {
		if strongParent != EmptyBlockID {
			o.strongChildCounters[strongParent]++
		}
	}
}

func (o *OrphanageManager) OrphanBlock(blockID BlockID, reason error) {
	o.Lock()
	defer o.Unlock()
	o.orphanBlockFutureCone(blockID, reason)
}
func (o *OrphanageManager) orphanBlockFutureCone(blockID BlockID, reason error) {
	futureConeWalker := walker.New[BlockID](false).Push(blockID)
	for futureConeWalker.HasNext() {
		blockID = futureConeWalker.Next()
		if reason == nil {
			reason = errors.Errorf("block %s orphaned because its past cone has been orphaned", blockID)
		}

		o.orphanBlock(blockID, reason)
		o.tangle.Storage.Children(blockID).Consume(func(approver *Child) {
			futureConeWalker.Push(approver.ChildBlockID())
		})
		reason = nil
	}
}

func (o *OrphanageManager) orphanBlock(blockID BlockID, reason error) {
	o.Events.BlockOrphaned.Trigger(&BlockOrphanedEvent{
		BlockID: blockID,
		Reason:  reason,
	})

	o.tangle.Storage.BlockMetadata(blockID).Consume(func(metadata *BlockMetadata) {
		metadata.SetOrphaned(true)
	})

	// remove blockID from unconfirmed block heap and from childStrongCounters map
	o.removeElementFromHeap(blockID)
	delete(o.strongChildCounters, blockID)

	o.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		for strongParent := range block.ParentsByType(StrongParentType) {
			if strongParent != EmptyBlockID {
				o.decreaseStrongChildCounter(strongParent)
			}
		}
	})
}

func (o *OrphanageManager) decreaseStrongChildCounter(blockID BlockID) {
	o.Lock()
	defer o.Unlock()
	if _, exists := o.strongChildCounters[blockID]; !exists {
		return
	}
	o.strongChildCounters[blockID]--

	if o.strongChildCounters[blockID] <= 0 {
		// do not remove block without any approvers from the heap here
		// as it still might get more approvers and be confirmed or orphaned due to TSC threshold
		delete(o.strongChildCounters, blockID)

		o.tangle.Storage.Block(blockID).Consume(func(block *Block) {
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
func (o *OrphanageManager) removeElementFromHeap(blockID BlockID) {
	for i := 0; i < len(o.unconfirmedBlocks); i++ {
		if o.unconfirmedBlocks[i].Value == blockID {
			heap.Remove(&o.unconfirmedBlocks, o.unconfirmedBlocks[i].index)
			break
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OrphanageManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

type OrphanageManagerEvents struct {
	BlockOrphaned       *event.Event[*BlockOrphanedEvent]
	AllChildrenOrphaned *event.Event[*Block]
}

type BlockOrphanedEvent struct {
	BlockID BlockID
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
