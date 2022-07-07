package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/event"
)

// region OrphanageManager /////////////////////////////////////////////////////////////////////////////////////////////

// OrphanageManager is a manager that tracks orphaned messages.
type OrphanageManager struct {
	Events *OrphanageManagerEvents

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
	o.tangle.Solidifier.Events.MessageSolid.Attach(event.NewClosure(func(event *MessageSolidEvent) {
		for strongParent := range event.Message.ParentsByType(StrongParentType) {
			o.increaseStrongChildCounter(strongParent)
		}
	}))

	o.tangle.ConfirmationOracle.Events().MessageAccepted.Attach(event.NewClosure(func(event *MessageAcceptedEvent) {
		o.Lock()
		defer o.Unlock()

		delete(o.strongChildCounters, event.Message.ID())
	}))
}

func (o *OrphanageManager) OrphanBlock(blockID MessageID, reason error) {
	o.tangle.Storage.Message(blockID).Consume(func(block *Message) {
		o.Events.BlockOrphaned.Trigger(&BlockOrphanedEvent{
			Block:  block,
			Reason: reason,
		})

		for strongParent := range block.ParentsByType(StrongParentType) {
			o.decreaseStrongChildCounter(strongParent)
		}
	})
}

func (o *OrphanageManager) increaseStrongChildCounter(blockID MessageID) {
	o.Lock()
	defer o.Unlock()

	o.strongChildCounters[blockID]++
}

func (o *OrphanageManager) decreaseStrongChildCounter(blockID MessageID) {
	o.Lock()
	defer o.Unlock()

	o.strongChildCounters[blockID]--

	if o.strongChildCounters[blockID] == 0 {
		delete(o.strongChildCounters, blockID)

		o.tangle.Storage.Message(blockID).Consume(func(block *Message) {
			o.Events.AllChildrenOrphaned.Trigger(block)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OrphanageManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

type OrphanageManagerEvents struct {
	BlockOrphaned       *event.Event[*BlockOrphanedEvent]
	AllChildrenOrphaned *event.Event[*Message]
}

type BlockOrphanedEvent struct {
	Block  *Message
	Reason error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
