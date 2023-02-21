package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test     *testing.T
	Instance *Tangle

	VirtualVoting *virtualvoting.TestFramework
	Booker        *booker.TestFramework
	Ledger        *ledger.TestFramework
	BlockDAG      *blockdag.TestFramework
	Votes         *votes.TestFramework
}

func NewTestTangle(t *testing.T, workers *workerpool.Group, ledger *ledger.Ledger, validators *sybilprotection.WeightedSet, optsTangle ...options.Option[Tangle]) *Tangle {
	storageInstance := blockdag.NewTestStorage(t, workers)

	tangle := New(workers, ledger, eviction.NewState(storageInstance), validators, func() epoch.Index {
		return 0
	}, func(id markers.SequenceID) markers.Index {
		return 1
	}, storageInstance.Commitments.Load,
		optsTangle...)

	return tangle
}

func NewTestFramework(test *testing.T, tangle *Tangle, virtualVotingTF *virtualvoting.TestFramework) *TestFramework {
	return &TestFramework{
		test:          test,
		Instance:      tangle,
		VirtualVoting: virtualVotingTF,
		Booker:        virtualVotingTF.Booker,
		Ledger:        virtualVotingTF.Ledger,
		BlockDAG:      virtualVotingTF.BlockDAG,
		Votes:         virtualVotingTF.Votes,
	}
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsTangle ...options.Option[Tangle]) *TestFramework {
	tangle := NewTestTangle(t, workers.CreateGroup("Tangle"),
		ledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB())),
		optsTangle...,
	)

	return NewTestFramework(t, tangle, virtualvoting.NewTestFramework(t, workers.CreateGroup("VirtualVotingTestFramework"),
		tangle.VirtualVoting,
	))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
