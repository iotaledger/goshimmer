package consensus

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/epochgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	Events *Events

	BlockGadget      *blockgadget.Gadget
	EpochGadget      *epochgadget.Gadget
	ConflictResolver *conflictresolver.ConflictResolver

	optsAcceptanceGadget        []options.Option[blockgadget.Gadget]
	optsEpochConfirmationGadget []options.Option[epochgadget.Gadget]
}

func New(workers *workerpool.Group, tangleInstance *tangle.Tangle, evictionState *eviction.State, lastConfirmedEpoch epoch.Index, totalWeightCallback func() int64, opts ...options.Option[Consensus]) *Consensus {
	return options.Apply(&Consensus{}, opts, func(c *Consensus) {
		c.BlockGadget = blockgadget.New(workers.CreateGroup("BlockGadget"), tangleInstance, evictionState, totalWeightCallback, c.optsAcceptanceGadget...)
		c.EpochGadget = epochgadget.New(workers.CreateGroup("EpochGadget"), tangleInstance, lastConfirmedEpoch, totalWeightCallback, c.optsEpochConfirmationGadget...)
		c.ConflictResolver = conflictresolver.New(tangleInstance.Ledger.ConflictDAG, tangleInstance.VirtualVoting.ConflictVotersTotalWeight)

		c.Events = NewEvents()
		c.Events.BlockGadget = c.BlockGadget.Events
		c.Events.EpochGadget = c.EpochGadget.Events
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptanceGadgetOptions(opts ...options.Option[blockgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsAcceptanceGadget = opts
	}
}

func WithEpochConfirmationGadgetOptions(opts ...options.Option[epochgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsEpochConfirmationGadget = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
