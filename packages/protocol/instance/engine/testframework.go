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
	models2 "github.com/iotaledger/goshimmer/packages/protocol/models"
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
	optsEvictionManager *eviction.Manager[models2.BlockID]
	optsValidatorSet    *validator.Set

	*TangleTestFramework
	*AcceptanceTestFramework
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
				t.optsEvictionManager = eviction.NewManager(0, models2.GenesisRootBlockProvider)
			}

			if t.optsValidatorSet == nil {
				t.optsValidatorSet = validator.NewSet()
			}

			t.Engine = New(time.Now(), t.optsLedger, t.optsEvictionManager, t.optsValidatorSet, t.optsEngineOptions...)
		}

		t.TangleTestFramework = tangle.NewTestFramework(test, tangle.WithTangle(t.Engine.Tangle))
		t.AcceptanceTestFramework = acceptance.NewTestFramework(test, acceptance.WithTangle(t.Engine.Tangle), acceptance.WithTangleTestFramework(t.TangleTestFramework))
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
