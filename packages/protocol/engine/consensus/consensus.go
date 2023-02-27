package consensus

import (
	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/slotgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	Events *Events

	BlockGadget      *blockgadget.Gadget
	SlotGadget       *slotgadget.Gadget
	ConflictResolver *conflictresolver.ConflictResolver

	optsAcceptanceGadget       []options.Option[blockgadget.Gadget]
	optsSlotConfirmationGadget []options.Option[slotgadget.Gadget]
}

func New(workers *workerpool.Group, tangleInstance *tangle.Tangle, evictionState *eviction.State, lastConfirmedSlot slot.Index, totalWeightCallback func() int64, opts ...options.Option[Consensus]) *Consensus {
	return options.Apply(&Consensus{}, opts, func(c *Consensus) {
		c.BlockGadget = blockgadget.New(workers.CreateGroup("BlockGadget"), tangleInstance, evictionState, tangleInstance.BlockDAG.SlotTimeProvider, totalWeightCallback, c.optsAcceptanceGadget...)
		c.SlotGadget = slotgadget.New(workers.CreateGroup("SlotGadget"), tangleInstance, lastConfirmedSlot, totalWeightCallback, c.optsSlotConfirmationGadget...)
		c.ConflictResolver = conflictresolver.New(tangleInstance.Ledger.ConflictDAG, tangleInstance.Booker.VirtualVoting.ConflictVotersTotalWeight)

		c.Events = NewEvents()
		c.Events.BlockGadget = c.BlockGadget.Events
		c.Events.SlotGadget = c.SlotGadget.Events
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptanceGadgetOptions(opts ...options.Option[blockgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsAcceptanceGadget = opts
	}
}

func WithSlotConfirmationGadgetOptions(opts ...options.Option[slotgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsSlotConfirmationGadget = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
