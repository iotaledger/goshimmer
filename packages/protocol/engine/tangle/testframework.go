package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Tangle *Tangle

	test *testing.T

	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsEvictionManager *eviction.Manager[models.BlockID]
	optsValidatorSet    *validator.Set
	optsTangle          []options.Option[Tangle]

	*VirtualVotingTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Tangle == nil {
			if t.optsLedger == nil {
				t.optsLedger = ledger.New(t.optsLedgerOptions...)
			}

			if t.optsEvictionManager == nil {
				t.optsEvictionManager = eviction.NewManager[models.BlockID]()
			}

			if t.optsValidatorSet == nil {
				t.optsValidatorSet = validator.NewSet()
			}

			t.Tangle = New(t.optsLedger, t.optsEvictionManager, t.optsValidatorSet, t.optsTangle...)
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

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(t *TestFramework) {
		if t.optsLedgerOptions != nil {
			panic("using the TestFramework with Ledger and LedgerOptions simultaneously is not allowed")
		}

		t.optsLedger = ledger
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		if t.optsLedger != nil {
			panic("using the TestFramework with Ledger and LedgerOptions simultaneously is not allowed")
		}

		t.optsLedgerOptions = opts
	}
}

func WithEvictionManager(evictionManager *eviction.Manager[models.BlockID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionManager = evictionManager
	}
}

func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidatorSet = validatorSet
	}
}

func WithTangle(tangle *Tangle) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Tangle = tangle
	}
}

func WithTangleOptions(opts ...options.Option[Tangle]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsTangle = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
