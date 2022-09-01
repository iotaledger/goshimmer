package consensus

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	*acceptance.Gadget

	optsAcceptanceGadget []options.Option[acceptance.Gadget]
}

func New(tangle *tangle.Tangle, opts ...options.Option[Consensus]) *Consensus {
	return options.Apply(new(Consensus), opts, func(c *Consensus) {
		c.Gadget = acceptance.New(tangle, c.optsAcceptanceGadget...)
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
