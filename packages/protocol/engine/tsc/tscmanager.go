package tsc

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region TSCManager /////////////////////////////////////////////////////////////////////////////////////////////

// TSCManager is a manager that tracks orphaned blocks.
type TSCManager struct {
	unconfirmedBlocks  TimedHeap
	tangle             *tangle.Tangle
	isBlockAccepted    func(models.BlockID) bool
	blockAcceptedEvent *event.Linkable[*acceptance.Block, acceptance.Events, *acceptance.Events]
	clock              *clock.Clock

	optsTimeSinceConfirmationThreshold time.Duration

	sync.Mutex
}

// New returns a new instance of TSCManager.
func New(isBlockAccepted func(models.BlockID) bool, blockAcceptedEvent *event.Linkable[*acceptance.Block, acceptance.Events, *acceptance.Events], tangle *tangle.Tangle, clock *clock.Clock, opts ...options.Option[TSCManager]) *TSCManager {
	return options.Apply(&TSCManager{
		isBlockAccepted:                    isBlockAccepted,
		blockAcceptedEvent:                 blockAcceptedEvent,
		clock:                              clock,
		tangle:                             tangle,
		optsTimeSinceConfirmationThreshold: time.Minute,
	}, opts, (*TSCManager).Setup)
}

func (o *TSCManager) Setup() {
	o.tangle.Events.Booker.BlockBooked.Attach(event.NewClosure(o.AddUnconfirmedBlock))

	// Handle this event synchronously to guarantee that confirmed block is removed from orphanage manager before
	// acceptance time is updated for this block as this could lead to some inconsistencies and manager trying to
	// orphan confirmed messages.
	o.blockAcceptedEvent.Hook(event.NewClosure(o.HandleAcceptedBlock))

	o.clock.Events.AcceptanceTimeUpdated.Hook(event.NewClosure(o.HandleTimeUpdate))

}

func (o *TSCManager) HandleTimeUpdate(evt *clock.TimeUpdate) {
	o.Lock()
	defer o.Unlock()
	o.orphanBeforeTSC(evt.NewTime.Add(-o.optsTimeSinceConfirmationThreshold))
}

func (o *TSCManager) HandleAcceptedBlock(acceptedBlock *acceptance.Block) {
	o.Lock()
	defer o.Unlock()

	// If block has been orphaned before acceptance, remove the flag from the block. Otherwise, remove the block from TimedHeap.
	if acceptedBlock.IsExplicitlyOrphaned() {
		o.tangle.SetOrphaned(acceptedBlock.Block.Block.Block, false)
	} else {
		o.removeElementFromHeap(acceptedBlock.Block.Block.Block)
	}
}

func (o *TSCManager) AddUnconfirmedBlock(block *booker.Block) {
	o.Lock()
	defer o.Unlock()

	heap.Push(&o.unconfirmedBlocks, &QueueElement{Value: block.Block, Key: block.IssuingTime()})
}

// orphanBeforeTSC removes the elements with key time earlier than the given time.
func (o *TSCManager) orphanBeforeTSC(minAllowedTime time.Time) {
	unconfirmedBlocksCount := o.unconfirmedBlocks.Len()
	for i := 0; i < unconfirmedBlocksCount; i++ {
		if o.unconfirmedBlocks[0].Key.After(minAllowedTime) {
			return
		}

		blockToOrphan := o.unconfirmedBlocks[0].Value
		o.removeElementFromHeap(blockToOrphan)
		if !o.isBlockAccepted(blockToOrphan.ID()) {
			o.tangle.SetOrphaned(blockToOrphan, true)
		}
	}
}

// removeElement removes the block from TSCManager
func (o *TSCManager) removeElementFromHeap(block *blockdag.Block) {
	for i := 0; i < len(o.unconfirmedBlocks); i++ {
		if o.unconfirmedBlocks[i].Value.ID() == block.ID() {
			heap.Remove(&o.unconfirmedBlocks, o.unconfirmedBlocks[i].index)
			break
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTimeSinceConfirmationThreshold(timeSinceConfirmationThreshold time.Duration) options.Option[TSCManager] {
	return func(o *TSCManager) {
		o.optsTimeSinceConfirmationThreshold = timeSinceConfirmationThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
