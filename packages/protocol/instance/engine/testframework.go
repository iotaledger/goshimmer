package engine

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TangleTestFramework = tangle.TestFramework
type AcceptanceTestFramework = acceptance.TestFramework

type TestFramework struct {
	Engine *Engine

	test *testing.T

	optsEngineOptions   []options.Option[Engine]
	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsEvictionManager *eviction.Manager[models.BlockID]
	optsValidatorSet    *validator.Set

	Tangle     *TangleTestFramework
	Acceptance *AcceptanceTestFramework
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
				t.optsEvictionManager = eviction.NewManager[models.BlockID]()
			}

			if t.optsValidatorSet == nil {
				t.optsValidatorSet = validator.NewSet()
			}

			t.Engine = New(time.Now(), t.optsLedger, t.optsEvictionManager, t.optsValidatorSet, t.optsEngineOptions...)
		}

		t.Tangle = tangle.NewTestFramework(test, tangle.WithTangle(t.Engine.Tangle))
		t.Acceptance = acceptance.NewTestFramework(test, acceptance.WithTangle(t.Engine.Tangle), acceptance.WithTangleTestFramework(t.Tangle))
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

func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidatorSet = validatorSet
	}
}

func WithEvictionManager(evictionManager *eviction.Manager[models.BlockID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionManager = evictionManager
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
