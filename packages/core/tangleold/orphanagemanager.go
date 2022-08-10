package tangleold

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// region OrphanageManager /////////////////////////////////////////////////////////////////////////////////////////////

// OrphanageManager is a manager that tracks orphaned blocks.
type OrphanageManager struct {
	Events *OrphanageManagerEvents

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
	o.tangle.Solidifier.Events.BlockSolid.Attach(event.NewClosure(func(event *BlockSolidEvent) {
		for strongParent := range event.Block.ParentsByType(StrongParentType) {
			o.increaseStrongChildCounter(strongParent)
		}
	}))

	o.tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *BlockAcceptedEvent) {
		o.Lock()
		defer o.Unlock()

		delete(o.strongChildCounters, event.Block.ID())
	}))
}

func (o *OrphanageManager) OrphanBlock(blockID BlockID, reason error) {
	o.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		o.Events.BlockOrphaned.Trigger(&BlockOrphanedEvent{
			Block:  block,
			Reason: reason,
		})

		for strongParent := range block.ParentsByType(StrongParentType) {
			o.decreaseStrongChildCounter(strongParent)
		}
	})
}

func (o *OrphanageManager) increaseStrongChildCounter(blockID BlockID) {
	o.Lock()
	defer o.Unlock()

	o.strongChildCounters[blockID]++
}

func (o *OrphanageManager) decreaseStrongChildCounter(blockID BlockID) {
	o.Lock()
	defer o.Unlock()

	o.strongChildCounters[blockID]--

	if o.strongChildCounters[blockID] == 0 {
		delete(o.strongChildCounters, blockID)

		o.tangle.Storage.Block(blockID).Consume(func(block *Block) {
			o.Events.AllChildrenOrphaned.Trigger(block)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OrphanageManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

type OrphanageManagerEvents struct {
	BlockOrphaned       *event.Event[*BlockOrphanedEvent]
	AllChildrenOrphaned *event.Event[*Block]
}

type BlockOrphanedEvent struct {
	Block  *Block
	Reason error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
