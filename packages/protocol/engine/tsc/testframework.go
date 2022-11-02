package tsc

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Manager        *Manager
	mockAcceptance *acceptance.MockAcceptanceGadget

	test *testing.T

	optsTSCManager          []options.Option[Manager]
	optsTangle              []options.Option[tangle.Tangle]
	optsIsBlockAcceptedFunc func(models.BlockID) bool
	optsBlockAcceptedEvent  *event.Linkable[*acceptance.Block]
	*tangle.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:           test,
		mockAcceptance: acceptance.NewMockAcceptanceGadget(),
	}, opts, func(t *TestFramework) {
		t.TestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangle...),
		)

		if t.optsIsBlockAcceptedFunc == nil {
			t.optsIsBlockAcceptedFunc = t.mockAcceptance.IsBlockAccepted
		}
		if t.optsBlockAcceptedEvent == nil {
			t.optsBlockAcceptedEvent = t.mockAcceptance.BlockAcceptedEvent
		}

		if t.Manager == nil {
			t.Manager = New(t.optsIsBlockAcceptedFunc, t.TestFramework.Tangle, t.optsTSCManager...)
		}

		t.TestFramework.Tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(t.Manager.AddBlock))
	})
}

func (t *TestFramework) AssertExplicitlyOrphaned(expectedState map[string]bool) {
	for alias, expectedOrphanage := range expectedState {
		t.BookerTestFramework.AssertBlock(alias, func(block *booker.Block) {
			assert.Equal(t.test, expectedOrphanage, block.IsExplicitlyOrphaned(), "block %s is incorrectly orphaned", block.ID())
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTSCManagerOptions(opts ...options.Option[Manager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTSCManager = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = opts
	}
}

func WithBlockAcceptedEvent(blockAcceptedEvent *event.Linkable[*acceptance.Block]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsBlockAcceptedEvent = blockAcceptedEvent
	}
}

func WithIsBlockAcceptedFunc(isBlockAcceptedFunc func(id models.BlockID) bool) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsIsBlockAcceptedFunc = isBlockAcceptedFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
