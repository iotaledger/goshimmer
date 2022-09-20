package tsc

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generalheap"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TSCManager /////////////////////////////////////////////////////////////////////////////////////////////

// TSCManager is a manager that tracks orphaned blocks.
type TSCManager struct {
	unconfirmedBlocks generalheap.Heap[timed.HeapKey, *blockdag.Block]
	tangle            *tangle.Tangle
	isBlockAccepted   func(models.BlockID) bool
	clock             *clock.Clock

	optsTimeSinceConfirmationThreshold time.Duration

	sync.Mutex
}

// New returns a new instance of TSCManager.
func New(isBlockAccepted func(models.BlockID) bool, tangle *tangle.Tangle, clock *clock.Clock, opts ...options.Option[TSCManager]) *TSCManager {
	return options.Apply(&TSCManager{
		isBlockAccepted:                    isBlockAccepted,
		clock:                              clock,
		tangle:                             tangle,
		optsTimeSinceConfirmationThreshold: time.Minute,
	}, opts, (*TSCManager).Setup)
}

func (o *TSCManager) Setup() {
	o.tangle.Events.Booker.BlockBooked.Attach(event.NewClosure(o.AddBlock))

	o.clock.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(o.HandleTimeUpdate))

}

func (o *TSCManager) HandleTimeUpdate(evt *clock.TimeUpdate) {
	o.Lock()
	defer o.Unlock()
	o.orphanBeforeTSC(evt.NewTime.Add(-o.optsTimeSinceConfirmationThreshold))
}

func (o *TSCManager) AddBlock(block *booker.Block) {
	o.Lock()
	defer o.Unlock()

	heap.Push(&o.unconfirmedBlocks, &generalheap.HeapElement[timed.HeapKey, *blockdag.Block]{Value: block.Block, Key: timed.HeapKey(block.IssuingTime())})
}

// orphanBeforeTSC removes all elements with key time earlier than the given time. If a block is not accepted by this time, it becomes orphaned.
func (o *TSCManager) orphanBeforeTSC(minAllowedTime time.Time) {
	unconfirmedBlocksCount := o.unconfirmedBlocks.Len()
	for i := 0; i < unconfirmedBlocksCount; i++ {
		if minAllowedTime.Before(time.Time(o.unconfirmedBlocks[0].Key)) {
			return
		}

		blockToOrphan := o.unconfirmedBlocks[0].Value
		heap.Pop(&o.unconfirmedBlocks)
		if !o.isBlockAccepted(blockToOrphan.ID()) {
			o.tangle.SetOrphaned(blockToOrphan, true)
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
