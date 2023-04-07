package testtangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag/inmemoryblockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type TestTangle struct {
	events   *tangle.Events
	blockDAG *inmemoryblockdag.BlockDAG
	booker   *markerbooker.Booker

	slotTimeProvider *slot.TimeProvider
	memPool          mempool.MemPool
	evictionState    *eviction.State
	validators       *sybilprotection.WeightedSet

	module.Module
}

func NewTestTangle(t *testing.T, workers *workerpool.Group, slotTimeProvider *slot.TimeProvider, memPool mempool.MemPool, validators *sybilprotection.WeightedSet, optsBooker ...options.Option[markerbooker.Booker]) *TestTangle {
	storageInstance := blockdag.NewTestStorage(t, workers)

	testTangle := &TestTangle{
		events:           tangle.NewEvents(),
		slotTimeProvider: slotTimeProvider,
		memPool:          memPool,
		evictionState:    eviction.NewState(storageInstance),
		validators:       validators,
	}

	testTangle.blockDAG = inmemoryblockdag.New(workers.CreateGroup("BlockDAG"), testTangle.evictionState, testTangle.SlotTimeProvider, storageInstance.Commitments.Load)
	testTangle.booker = markerbooker.New(workers.CreateGroup("Booker"), testTangle.evictionState, memPool, validators, testTangle.SlotTimeProvider,
		append([]options.Option[markerbooker.Booker]{
			markerbooker.WithSlotCutoffCallback(func() slot.Index {
				return 0
			}),
			markerbooker.WithSequenceCutoffCallback(func(id markers.SequenceID) markers.Index {
				return 1
			}),
		}, optsBooker...)...,
	)
	testTangle.booker.Initialize(testTangle.blockDAG)

	testTangle.TriggerConstructed()
	testTangle.TriggerInitialized()

	return testTangle
}

var _ tangle.Tangle = new(TestTangle)

func (t *TestTangle) Events() *tangle.Events {
	return t.events
}

func (t *TestTangle) Booker() booker.Booker {
	return t.booker
}

func (t *TestTangle) BlockDAG() blockdag.BlockDAG {
	return t.blockDAG
}

func (t *TestTangle) MemPool() mempool.MemPool {
	return t.memPool
}

func (t *TestTangle) EvictionState() *eviction.State {
	return t.evictionState
}

func (t *TestTangle) SlotTimeProvider() *slot.TimeProvider {
	return t.slotTimeProvider
}

func (t *TestTangle) Validators() *sybilprotection.WeightedSet {
	return t.validators
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, memPool mempool.MemPool, slotTimeProvider *slot.TimeProvider, optsBooker ...options.Option[markerbooker.Booker]) *tangle.TestFramework {
	validators := sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))

	testTangle := NewTestTangle(t, workers.CreateGroup("Tangle"),
		slotTimeProvider,
		memPool,
		validators,
		optsBooker...,
	)

	return tangle.NewTestFramework(t, testTangle, booker.NewTestFramework(t, workers.CreateGroup("BookerTestFramework"),
		testTangle.booker,
		testTangle.blockDAG,
		testTangle.memPool,
		validators,
		testTangle.SlotTimeProvider,
	))
}
