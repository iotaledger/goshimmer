package tsc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test           *testing.T
	Manager        *Manager
	MockAcceptance *blockgadget.MockAcceptanceGadget

	Tangle        *tangle.TestFramework
	BlockDAG      *blockdag.TestFramework
	Booker        *booker.TestFramework
	VirtualVoting *virtualvoting.TestFramework
}

func NewTestFramework(test *testing.T, tangleTF *tangle.TestFramework, optsTSCManager ...options.Option[Manager]) *TestFramework {
	t := &TestFramework{
		test:           test,
		Tangle:         tangleTF,
		BlockDAG:       tangleTF.BlockDAG,
		Booker:         tangleTF.Booker,
		VirtualVoting:  tangleTF.VirtualVoting,
		MockAcceptance: blockgadget.NewMockAcceptanceGadget(),
	}

	t.Manager = New(t.MockAcceptance.IsBlockAccepted, tangleTF.Instance, optsTSCManager...)
	event.Hook(t.Tangle.Booker.Instance.Events.BlockBooked, func(event *booker.BlockBookedEvent) {
		t.Manager.AddBlock(event.Block)
	})

	return t
}

func (t *TestFramework) AssertOrphaned(expectedState map[string]bool) {
	for alias, expectedOrphanage := range expectedState {
		t.Tangle.Booker.AssertBlock(alias, func(block *booker.Block) {
			require.Equal(t.test, expectedOrphanage, block.Block.IsOrphaned(), "block %s is incorrectly orphaned", block.ID())
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
