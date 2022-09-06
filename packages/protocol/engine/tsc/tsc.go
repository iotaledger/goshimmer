package tsc

import (
	"container/heap"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
)

// region OrphanageManager /////////////////////////////////////////////////////////////////////////////////////////////

// OrphanageManager is a manager that tracks orphaned blocks.
type OrphanageManager struct {
	unconfirmedBlocks TimedHeap
	tangle            *tangle.Tangle
	gadget            *acceptance.Gadget
	clock             *engine.Clock
	sync.Mutex
}

// NewOrphanageManager returns a new OrphanageManager.
func NewOrphanageManager(tangle *tangle.Tangle, gadget *acceptance.Gadget, clock *engine.Clock) *OrphanageManager {
	return &OrphanageManager{
		gadget: gadget,
		clock:  clock,
		tangle: tangle,
	}
}

func (o *OrphanageManager) Setup() {
	o.tangle.Events.Booker.BlockBooked.Attach(event.NewClosure(o.addUnconfirmedBlock))

	// Handle this event synchronously to guarantee that confirmed block is removed from orphanage manager before
	// acceptance time is updated for this block as this could lead to some inconsistencies and manager trying to
	// orphan confirmed messages.
	o.gadget.Events.BlockAccepted.Hook(event.NewClosure(func(evt *acceptance.Block) {
		o.Lock()
		defer o.Unlock()

		o.removeElementFromHeap(evt.Block.ID())

		// if block has been orphaned before acceptance, remove the flag from the block and it's future cone if it satisfies the TSC threshold. Also add tips if possible
		o.deorphanBlockFutureCone(evt.Block.ID())
	}))

	o.clock.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(evt *TimeUpdate) {
		o.Lock()
		defer o.Unlock()
		o.orphanBeforeTSC(evt.ATT.Add(-o.tangle.Options.TimeSinceConfirmationThreshold))
	}))

}

func (o *OrphanageManager) addUnconfirmedBlock(block *booker.Block) {
	o.Lock()
	defer o.Unlock()
	heap.Push(&o.unconfirmedBlocks, &QueueElement{Value: block.ID(), Key: block.IssuingTime()})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
