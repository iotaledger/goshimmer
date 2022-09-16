package tsc

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/clock"
	acceptance2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/consensus/acceptance"
	tangle2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/models"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	OrphanageManager *TSCManager
	mockAcceptance   *acceptance2.MockAcceptanceGadget

	test *testing.T

	optsTSCManager          []options.Option[TSCManager]
	optsTangle              []options.Option[tangle2.Tangle]
	optsIsBlockAcceptedFunc func(models.BlockID) bool
	optsBlockAcceptedEvent  *event.Linkable[*acceptance2.Block, acceptance2.Events, *acceptance2.Events]
	optsClock               *clock.Clock
	*tangle2.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:           test,
		mockAcceptance: acceptance2.NewMockAcceptanceGadget(),
	}, opts, func(t *TestFramework) {
		t.TestFramework = tangle2.NewTestFramework(
			test,
			tangle2.WithTangleOptions(t.optsTangle...),
		)

		if t.optsIsBlockAcceptedFunc == nil {
			t.optsIsBlockAcceptedFunc = t.mockAcceptance.IsBlockAccepted
		}
		if t.optsBlockAcceptedEvent == nil {
			t.optsBlockAcceptedEvent = t.mockAcceptance.BlockAcceptedEvent
		}
		if t.optsClock == nil {
			t.optsClock = clock.NewClock(time.Now().Add(-5 * time.Hour))
		}

		if t.OrphanageManager == nil {
			t.OrphanageManager = New(t.optsIsBlockAcceptedFunc, t.TestFramework.Tangle, t.optsClock, t.optsTSCManager...)
		}

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

func WithTSCManagerOptions(opts ...options.Option[TSCManager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTSCManager = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle2.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = opts
	}
}

func WithBlockAcceptedEvent(blockAcceptedEvent *event.Linkable[*acceptance2.Block, acceptance2.Events, *acceptance2.Events]) options.Option[TestFramework] {
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
