package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/hive.go/core/slot"
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
	Ledger        *mempool.TestFramework
	BlockDAG      *blockdag.TestFramework
	Votes         *votes.TestFramework
}

func NewTestTangle(t *testing.T, workers *workerpool.Group, slotTimeProvider *slot.TimeProvider, ledger mempool.MemPool, validators *sybilprotection.WeightedSet, optsTangle ...options.Option[Tangle]) *Tangle {
	storageInstance := blockdag.NewTestStorage(t, workers)

	tangle := New(workers, ledger, eviction.NewState(storageInstance),
		func() *slot.TimeProvider {
			return slotTimeProvider
		},
		validators,
		func() slot.Index {
			return 0
		},
		func(id markers.SequenceID) markers.Index {
			return 1
		},
		storageInstance.Commitments.Load,
		optsTangle...)

	return tangle
}

func NewTestFramework(test *testing.T, tangle *Tangle, bookerTF *booker.TestFramework) *TestFramework {
	return &TestFramework{
		test:          test,
		Instance:      tangle,
		Booker:        bookerTF,
		VirtualVoting: bookerTF.VirtualVoting,
		Ledger:        bookerTF.Ledger,
		BlockDAG:      bookerTF.BlockDAG,
		Votes:         bookerTF.VirtualVoting.Votes,
	}
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, ledger mempool.MemPool, slotTimeProvider *slot.TimeProvider, optsTangle ...options.Option[Tangle]) *TestFramework {
	tangle := NewTestTangle(t, workers.CreateGroup("Tangle"),
		slotTimeProvider,
		ledger,
		sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB())),
		optsTangle...,
	)

	return NewTestFramework(t, tangle, booker.NewTestFramework(t, workers.CreateGroup("BookerTestFramework"),
		tangle.Booker,
	))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
