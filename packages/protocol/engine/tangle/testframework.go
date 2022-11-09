package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
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
	optsValidatorSet  *validator.Set
	optsTangle        []options.Option[Tangle]

	*VirtualVotingTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Tangle == nil {
			storage := storage.New(test.TempDir(), 1)
			test.Cleanup(func() {
				t.optsLedger.Shutdown()
				if err := storage.Shutdown(); err != nil {
					test.Fatal(err)
				}
			})

			if t.optsLedger == nil {
				t.optsLedger = ledger.New(storage, t.optsLedgerOptions...)
			}

			if t.optsEvictionState == nil {
				t.optsEvictionState = eviction.NewState(storage)
			}

			if t.optsValidatorSet == nil {
				t.optsValidatorSet = validator.NewSet()
			}

			t.Tangle = New(t.optsLedger, t.optsEvictionState, t.optsValidatorSet, t.optsTangle...)
		}

		t.VirtualVotingTestFramework = virtualvoting.NewTestFramework(
			test,
			virtualvoting.WithBlockDAG(t.Tangle.BlockDAG),
			virtualvoting.WithLedger(t.Tangle.Ledger),
			virtualvoting.WithBooker(t.Tangle.Booker),
			virtualvoting.WithVirtualVoting(t.Tangle.VirtualVoting),
		)
	})
}

type VirtualVotingTestFramework = virtualvoting.TestFramework

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

// WithValidatorSet sets the validator set that is used by the Tangle.
func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidatorSet = validatorSet
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
