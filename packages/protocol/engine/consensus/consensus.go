package consensus

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	Events *Events

	*acceptance.Gadget
	*conflictresolver.ConflictResolver

	optsAcceptanceGadget []options.Option[acceptance.Gadget]
}

func New(tangle *tangle.Tangle, evictionState *eviction.State, opts ...options.Option[Consensus]) *Consensus {
	return options.Apply(new(Consensus), opts, func(c *Consensus) {
		c.Gadget = acceptance.New(tangle, evictionState, c.optsAcceptanceGadget...)
		c.ConflictResolver = conflictresolver.New(tangle.Ledger.ConflictDAG, func(conflictID utxo.TransactionID) (weight int64) {
			return tangle.VirtualVoting.ConflictVoters(conflictID).TotalWeight()
		})

		c.Events = NewEvents()
		c.Events.Acceptance = c.Gadget.Events
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptanceGadgetOptions(opts ...options.Option[acceptance.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsAcceptanceGadget = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
