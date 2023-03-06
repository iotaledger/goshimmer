package tangle

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a conflict free replicated data type that allows users to issue their own Blocks with each Block casting
// virtual votes on existing conflicts.
type Tangle struct {
	Events *Events

	BlockDAG *blockdag.BlockDAG
	Booker   *booker.Booker
	Ledger   ledger.Ledger

	optsBlockDAG []options.Option[blockdag.BlockDAG]
	optsBooker   []options.Option[booker.Booker]
}

// New is the constructor for a new Tangle.
func New(
	workers *workerpool.Group,
	ledger ledger.Ledger,
	evictionState *eviction.State,
	slotTimeProviderFunc func() *slot.TimeProvider,
	validators *sybilprotection.WeightedSet,
	slotCutoffCallback func() slot.Index,
	sequenceCutoffCallback func(id markers.SequenceID) markers.Index,
	commitmentFunc func(slot.Index) (*commitment.Commitment, error),
	opts ...options.Option[Tangle],
) (newTangle *Tangle) {
	return options.Apply(new(Tangle), opts, func(t *Tangle) {
		t.Ledger = ledger
		t.BlockDAG = blockdag.New(workers.CreateGroup("BlockDAG"), evictionState, slotTimeProviderFunc, commitmentFunc, t.optsBlockDAG...)
		t.Booker = booker.New(workers.CreateGroup("Booker"), t.BlockDAG, ledger, validators,
			append(t.optsBooker,
				booker.WithVirtualVotingOptions(
					virtualvoting.WithSlotCutoffCallback(slotCutoffCallback),
					virtualvoting.WithSequenceCutoffCallback(sequenceCutoffCallback),
				))...)

		t.Events = NewEvents()
		t.Events.BlockDAG = t.BlockDAG.Events
		t.Events.Booker = t.Booker.Events
	})
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
