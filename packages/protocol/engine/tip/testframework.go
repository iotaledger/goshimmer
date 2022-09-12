package tip

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	TipManager     *TipManager
	mockAcceptance *acceptance.MockAcceptanceGadget

	test *testing.T

	optsTipManager          []options.Option[TipManager]
	optsTangle              []options.Option[tangle.Tangle]
	optsIsBlockAcceptedFunc func(models.BlockID) bool

	optsClock *clock.Clock
	*tangle.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
		mockAcceptance: &acceptance.MockAcceptanceGadget{
			BlockAcceptedEvent: event.NewLinkable[*acceptance.Block, acceptance.Events, *acceptance.Events](),
			AcceptedBlocks:     make(map[models.BlockID]bool),
		},
	}, opts, func(t *TestFramework) {
		t.TestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangle...),
		)

		if t.optsIsBlockAcceptedFunc == nil {
			t.optsIsBlockAcceptedFunc = t.mockAcceptance.IsBlockAccepted
		}
		if t.optsClock == nil {
			t.optsClock = clock.NewClock(time.Now().Add(-5 * time.Hour))
		}

		if t.TipManager == nil {
			t.TipManager = NewTipManager(t.TestFramework.Tangle, t.optsTipManager...)
		}

	})
}

func (t *TestFramework) AssertTips(actualTips scheduler.Blocks, expectedStateAliases ...string) {
	actualTipsIDs := models.NewBlockIDs()

	for it := actualTips.Iterator(); it.HasNext(); {
		actualTipsIDs.Add(it.Next().ID())
	}

	assert.Equal(t.test, actualTips.Size(), len(expectedStateAliases), "amount of tips=%d does not match expected=%d", len(expectedStateAliases), actualTips.Size())
	for _, expectedBlockAlias := range expectedStateAliases {
		t.VirtualVotingTestFramework.AssertBlock(expectedBlockAlias, func(block *virtualvoting.Block) {
			assert.True(t.test, actualTipsIDs.Contains(block.ID()), "block %s is not in the selected tips", block.ID())
		})
	}
}

func (t *TestFramework) AssertTipCount(expectedTipCount int) {
	assert.Equal(t.test, expectedTipCount, t.TipManager.TipCount(), "amount of tips=%d does not match expected=%d", t.TipManager.TipCount(), expectedTipCount)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTipManagerOptions(opts ...options.Option[TipManager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTipManager = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = opts
	}
}

func WithIsBlockAcceptedFunc(isBlockAcceptedFunc func(id models.BlockID) bool) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsIsBlockAcceptedFunc = isBlockAcceptedFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
