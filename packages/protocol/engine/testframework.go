package engine

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type (
	TangleTestFramework     = tangle.TestFramework
	AcceptanceTestFramework = acceptance.TestFramework
)

type TestFramework struct {
	Engine *Engine

	test *testing.T

	optsEngineOptions []options.Option[Engine]

	Tangle     *TangleTestFramework
	Acceptance *AcceptanceTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	chainStorage := storage.New(test.TempDir(), 1)
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Engine == nil {
			t.Engine = New(chainStorage, t.optsEngineOptions...)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
