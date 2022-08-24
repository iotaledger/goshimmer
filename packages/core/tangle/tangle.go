package tangle

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/otv"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a conflict free replicated data type that allows users to issue their own Blocks.
type Tangle struct {
	*blockdag.BlockDAG
	*booker.Booker
	*otv.OnTangleVoting
	*ledger.Ledger

	optsLedger   []options.Option[ledger.Ledger]
	optsBlockDAG []options.Option[blockdag.BlockDAG]
	optsBooker   []options.Option[booker.Booker]
	optsOTV      []options.Option[otv.OnTangleVoting]
}

// New is the constructor for a new Tangle.
func New(validatorSet *validator.Set, evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(new(Tangle), opts, func(t *Tangle) {
		t.Ledger = ledger.New( /* TODO CHANGE LEDGER OPTIONS TO GENERIC OPTS */ )
		t.BlockDAG = blockdag.New(evictionManager, t.optsBlockDAG...)
		t.Booker = booker.New(evictionManager, t.Ledger, t.BlockDAG, t.optsBooker...)
		t.OnTangleVoting = otv.New(evictionManager, t.Ledger, t.BlockDAG, t.Booker, validatorSet, t.optsOTV...)
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

// WithOTVOptions returns an Option for the Tangle that allows to pass in Options for the virtual voting mechanism.
func WithOTVOptions(opts ...options.Option[otv.OnTangleVoting]) options.Option[Tangle] {
	return func(tangle *Tangle) {
		tangle.optsOTV = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
