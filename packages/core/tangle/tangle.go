package tangle

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a conflict free replicated data type that allows users to issue their own Blocks with each Block casting
// virtual votes on existing conflicts.
type Tangle struct {
	*blockdag.BlockDAG
	*ledger.Ledger
	*booker.Booker
	*virtualvoting.VirtualVoting

	optsBlockDAG      []options.Option[blockdag.BlockDAG]
	optsLedger        []options.Option[ledger.Ledger]
	optsBooker        []options.Option[booker.Booker]
	optsVirtualVoting []options.Option[virtualvoting.VirtualVoting]
}

// New is the constructor for a new Tangle.
func New(evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(new(Tangle), opts, func(t *Tangle) {
		t.BlockDAG = blockdag.New(evictionManager, t.optsBlockDAG...)
		t.Ledger = ledger.New(t.optsLedger...)
		t.Booker = booker.New(t.BlockDAG, t.Ledger, t.optsBooker...)
		t.VirtualVoting = virtualvoting.New(t.Booker, validatorSet, t.optsVirtualVoting...)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithLedgerOptions returns an Option for the Tangle that allows to pass in Options for the Ledger.
func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsLedger = opts
	}
}

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
