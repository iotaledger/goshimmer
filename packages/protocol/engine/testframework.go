package engine

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Engine     *Engine
	Tangle     *tangle.TestFramework
	Acceptance *acceptance.TestFramework

	test *testing.T

	optsEngineOptions   []options.Option[Engine]
	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsEvictionManager *eviction.Manager[models.BlockID]
	optsValidatorSet    *validator.Set
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Engine == nil {
			if t.optsLedger == nil {
				t.optsLedger = ledger.New(t.optsLedgerOptions...)
			}

			if t.optsEvictionManager == nil {
				t.optsEvictionManager = eviction.NewManager(models.IsEmptyBlockID)
			}

			if t.optsValidatorSet == nil {
				t.optsValidatorSet = validator.NewSet()
			}

			t.Engine = New(t.optsLedger, t.optsEvictionManager, t.optsValidatorSet, t.optsEngineOptions...)
		}

		t.Tangle = tangle.NewTestFramework(test, tangle.WithTangle(t.Engine.Tangle))
		t.Acceptance = acceptance.NewTestFramework(test, acceptance.WithTangle(t.Engine.Tangle))
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngine(engine *Engine) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Engine = engine
	}
}

func WithEngineOptions(opts ...options.Option[Engine]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEngineOptions = opts
	}
}

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedger = ledger
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedgerOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
