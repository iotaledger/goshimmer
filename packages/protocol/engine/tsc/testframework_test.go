package tsc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test           *testing.T
	Manager        *tsc.Manager
	MockAcceptance *blockgadget.MockBlockGadget

	Tangle        *tangle.TestFramework
	BlockDAG      *blockdag.TestFramework
	Booker        *booker.TestFramework
	VirtualVoting *booker.VirtualVotingTestFramework
}

func NewTestFramework(test *testing.T, tangleTF *tangle.TestFramework, optsTSCManager ...options.Option[tsc.Manager]) *TestFramework {
	t := &TestFramework{
		test:           test,
		Tangle:         tangleTF,
		BlockDAG:       tangleTF.BlockDAG,
		Booker:         tangleTF.Booker,
		VirtualVoting:  tangleTF.VirtualVoting,
		MockAcceptance: blockgadget.NewMockAcceptanceGadget(),
	}

	t.Manager = tsc.New(t.MockAcceptance.IsBlockAccepted, tangleTF.Instance, optsTSCManager...)
	t.Tangle.Booker.Instance.Events().BlockBooked.Hook(func(event *booker.BlockBookedEvent) {
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
