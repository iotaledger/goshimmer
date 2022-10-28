package consensus

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/epochconfirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	Events *Events

	AcceptanceGadget        *acceptance.Gadget
	EpochConfirmationGadget *epochconfirmation.Gadget
	*conflictresolver.ConflictResolver

	optsAcceptanceGadget        []options.Option[acceptance.Gadget]
	optsEpochConfirmationGadget []options.Option[epochconfirmation.Gadget]
}

func New(tangle *tangle.Tangle, lastConfirmedEpoch epoch.Index, totalWeightCallback func() (int64, error), opts ...options.Option[Consensus]) *Consensus {
	return options.Apply(new(Consensus), opts, func(c *Consensus) {
		c.AcceptanceGadget = acceptance.New(tangle, c.optsAcceptanceGadget...)
		c.EpochConfirmationGadget = epochconfirmation.New(tangle, lastConfirmedEpoch, totalWeightCallback, c.optsEpochConfirmationGadget...)
		c.ConflictResolver = conflictresolver.New(tangle.Ledger.ConflictDAG, func(conflictID utxo.TransactionID) (weight int64) {
			return tangle.VirtualVoting.ConflictVoters(conflictID).TotalWeight()
		})

		c.Events = NewEvents()
		c.Events.Acceptance = c.AcceptanceGadget.Events
		c.Events.EpochConfirmation = c.EpochConfirmationGadget.Events
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptanceGadgetOptions(opts ...options.Option[acceptance.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsAcceptanceGadget = opts
	}
}

func WithEpochConfirmationGadgetOptions(opts ...options.Option[epochconfirmation.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsEpochConfirmationGadget = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
