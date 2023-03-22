package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
)

type TestFramework struct {
	test     *testing.T
	Instance Tangle

	VirtualVoting *booker.VirtualVotingTestFramework
	Booker        *booker.TestFramework
	MemPool       *mempool.TestFramework
	BlockDAG      *blockdag.TestFramework
	Votes         *votes.TestFramework
}

func NewTestFramework(test *testing.T, tangle Tangle, bookerTF *booker.TestFramework) *TestFramework {
	return &TestFramework{
		test:          test,
		Instance:      tangle,
		Booker:        bookerTF,
		VirtualVoting: bookerTF.VirtualVoting,
		MemPool:       bookerTF.Ledger,
		BlockDAG:      bookerTF.BlockDAG,
		Votes:         bookerTF.VirtualVoting.Votes,
	}
}
