package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Tangle *Tangle

	test *testing.T

	optsLedger        *ledger.Ledger
	optsLedgerOptions []options.Option[ledger.Ledger]
	optsEvictionState *eviction.State
	optsTangle        []options.Option[Tangle]
	optsValidators    *sybilprotection.WeightedSet

	*VirtualVotingTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Tangle == nil {
			storageInstance := storage.New(test.TempDir(), 1)
			test.Cleanup(func() {
				t.optsLedger.Shutdown()
				storageInstance.Shutdown()
			})

			if t.optsLedger == nil {
				t.optsLedger = ledger.New(storageInstance, t.optsLedgerOptions...)
			}

			if t.optsEvictionState == nil {
				t.optsEvictionState = eviction.NewState(storageInstance)
			}

			if t.optsValidators == nil {
				t.optsValidators = sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))
			}

			t.Tangle = New(t.optsLedger, t.optsEvictionState, t.optsValidators, func() epoch.Index {
				return 0
			}, func(id markers.SequenceID) markers.Index {
				return 1
			}, t.optsTangle...)
		}

		t.VirtualVotingTestFramework = virtualvoting.NewTestFramework(
			test,
			virtualvoting.WithBlockDAG(t.Tangle.BlockDAG),
			virtualvoting.WithLedger(t.Tangle.Ledger),
			virtualvoting.WithBooker(t.Tangle.Booker),
			virtualvoting.WithVirtualVoting(t.Tangle.VirtualVoting),
			virtualvoting.WithValidators(t.Tangle.Validators),
		)
	})
}

type VirtualVotingTestFramework = virtualvoting.TestFramework

func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	t.VirtualVotingTestFramework.WaitUntilAllTasksProcessed()

	return t
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithLedger sets the ledger that is used by the Tangle.
func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(t *TestFramework) {
		if t.optsLedgerOptions != nil {
			panic("using the TestFramework with Ledger and LedgerOptions simultaneously is not allowed")
		}

		t.optsLedger = ledger
	}
}

// WithLedgerOptions sets the ledger options that are used to create the ledger that is used by the Tangle.
func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		if t.optsLedger != nil {
			panic("using the TestFramework with Ledger and LedgerOptions simultaneously is not allowed")
		}

		t.optsLedgerOptions = opts
	}
}

// WithEvictionState sets the eviction state that is used by the Tangle.
func WithEvictionState(evictionState *eviction.State) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionState = evictionState
	}
}

// WithTangle sets the Tangle that is used by the TestFramework.
func WithTangle(tangle *Tangle) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Tangle = tangle
	}
}

// WithTangleOptions sets the Tangle options that are used to create the Tangle that is used by the TestFramework.
func WithTangleOptions(opts ...options.Option[Tangle]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsTangle = opts
	}
}

// WithValidators sets the Tangle options to receive a set of validators.
func WithValidators(validators *sybilprotection.WeightedSet) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidators = validators
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
