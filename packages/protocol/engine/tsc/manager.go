package tsc

import (
	"container/heap"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/generalheap"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/timed"
)

// region Manager /////////////////////////////////////////////////////////////////////////////////////////////

// Manager is a manager that tracks orphaned blocks.
type Manager struct {
	unacceptedBlocks generalheap.Heap[timed.HeapKey, *blockdag.Block]
	tangle           tangle.Tangle
	isBlockAccepted  func(models.BlockID) bool

	optsTimeSinceConfirmationThreshold time.Duration

	sync.Mutex
}

// New returns a new instance of Manager.
func New(isBlockAccepted func(models.BlockID) bool, tangle tangle.Tangle, opts ...options.Option[Manager]) *Manager {
	return options.Apply(&Manager{
		isBlockAccepted:                    isBlockAccepted,
		tangle:                             tangle,
		optsTimeSinceConfirmationThreshold: time.Minute,
	}, opts)
}

func (o *Manager) HandleTimeUpdate(newTime time.Time) {
	o.Lock()
	defer o.Unlock()

	o.orphanBeforeTSC(newTime.Add(-o.optsTimeSinceConfirmationThreshold))
}

func (o *Manager) AddBlock(block *booker.Block) {
	o.Lock()
	defer o.Unlock()

	heap.Push(&o.unacceptedBlocks, &generalheap.HeapElement[timed.HeapKey, *blockdag.Block]{Value: block.Block, Key: timed.HeapKey(block.IssuingTime())})
}

// orphanBeforeTSC removes all elements with key time earlier than the given time. If a block is not accepted by this time, it becomes orphaned.
func (o *Manager) orphanBeforeTSC(minAllowedTime time.Time) {
	unacceptedBlocksCount := o.unacceptedBlocks.Len()
	for i := 0; i < unacceptedBlocksCount; i++ {
		if minAllowedTime.Before(time.Time(o.unacceptedBlocks[0].Key)) {
			return
		}

		blockToOrphan := o.unacceptedBlocks[0].Value
		heap.Pop(&o.unacceptedBlocks)
		if !o.isBlockAccepted(blockToOrphan.ID()) {
			o.tangle.BlockDAG().SetOrphaned(blockToOrphan, true)
		}
	}
}

func (o *Manager) Size() int {
	o.Lock()
	defer o.Unlock()

	return o.unacceptedBlocks.Len()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTimeSinceConfirmationThreshold(timeSinceConfirmationThreshold time.Duration) options.Option[Manager] {
	return func(o *Manager) {
		o.optsTimeSinceConfirmationThreshold = timeSinceConfirmationThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
