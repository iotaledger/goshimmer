package markerbooker

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag/inmemoryblockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, ledger mempool.MemPool, optsBooker ...options.Option[Booker]) *booker.TestFramework {
	storageInstance := blockdag.NewTestStorage(t, workers)
	evictionState := eviction.NewState(storageInstance)
	slotTimeProvider := slot.NewTimeProvider(time.Now().Unix(), 10)
	blockDAG := inmemoryblockdag.NewTestBlockDAG(t, workers, evictionState, slotTimeProvider, blockdag.DefaultCommitmentFunc)

	validators := sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))

	markerBooker := New(
		workers.CreateGroup("Booker"),
		evictionState,
		ledger,
		validators,
		blockDAG.SlotTimeProvider,
		optsBooker...,
	)
	markerBooker.Initialize(blockDAG)

	return booker.NewTestFramework(t, workers, markerBooker, blockDAG, ledger, validators, func() *slot.TimeProvider {
		return slotTimeProvider
	})
}
