package tangle

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a conflict free replicated data type that allows users to issue their own Blocks with each Block casting
// virtual votes on existing conflicts.
type Tangle struct {
	optsBlockDAG      []options.Option[blockdag.BlockDAG]
	optsBooker        []options.Option[booker.Booker]
	optsVirtualVoting []options.Option[virtualvoting.VirtualVoting]

	*blockdag.BlockDAG
	*booker.Booker
	*virtualvoting.VirtualVoting
}

// New is the constructor for a new Tangle.
func New(ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(new(Tangle), opts, func(t *Tangle) {
		t.BlockDAG = blockdag.New(evictionManager, t.optsBlockDAG...)
		t.Booker = booker.New(t.BlockDAG, ledger, t.optsBooker...)
		t.VirtualVoting = virtualvoting.New(t.Booker, validatorSet, t.optsVirtualVoting...)
	})
}

func (t *Tangle) Block(blockID models.BlockID) (block *virtualvoting.Block, exists bool) {
	return t.VirtualVoting.Block(blockID)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithBlockDAGOptions returns an Option for the Tangle that allows to pass in Options for the BlockDAG.
func WithBlockDAGOptions(opts ...options.Option[blockdag.BlockDAG]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsBlockDAG = opts
	}
}

// WithBookerOptions returns an Option for the Tangle that allows to pass in Options for the Booker.
func WithBookerOptions(opts ...options.Option[booker.Booker]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsBooker = opts
	}
}

// WithVirtualVotingOptions returns an Option for the Tangle that allows to pass in Options for the virtual voting
// mechanism.
func WithVirtualVotingOptions(opts ...options.Option[virtualvoting.VirtualVoting]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsVirtualVoting = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
