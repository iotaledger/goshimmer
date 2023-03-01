package tipmanager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Instance *TipManager
	Engine   *engine.Engine
	Tangle   *tangle.TestFramework

	mockAcceptance       *blockgadget.MockAcceptanceGadget
	scheduledBlocks      *shrinkingmap.ShrinkingMap[models.BlockID, *scheduler.Block]
	scheduledBlocksMutex sync.RWMutex

	test       *testing.T
	tipAdded   uint32
	tipRemoved uint32

	optsTipManagerOptions []options.Option[TipManager]
	optsEngineOptions     []options.Option[engine.Engine]
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, slotTimeProvider *slot.TimeProvider, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:            test,
		mockAcceptance:  blockgadget.NewMockAcceptanceGadget(),
		scheduledBlocks: shrinkingmap.New[models.BlockID, *scheduler.Block](),
	}, opts, func(t *TestFramework) {
		storageInstance := blockdag.NewTestStorage(test, workers)
		// set MinCommittableSlotAge to genesis so nothing is committed.
		t.Engine = engine.New(workers.CreateGroup("Engine"), storageInstance, dpos.NewProvider(), mana1.NewProvider(), slotTimeProvider, t.optsEngineOptions...)

		test.Cleanup(func() {
			t.Engine.Shutdown()
			workers.WaitChildren()
			storageInstance.Shutdown()
		})

		t.Tangle = tangle.NewTestFramework(
			test,
			t.Engine.Tangle,
			booker.NewTestFramework(test, workers.CreateGroup("BookerTestFramework"), t.Engine.Tangle.Booker),
		)

		t.Instance = New(workers.CreateGroup("TipManager"), slotTimeProvider, t.mockSchedulerBlock, t.optsTipManagerOptions...)
		t.Instance.LinkTo(t.Engine)

		t.Instance.blockAcceptanceGadget = t.mockAcceptance

		t.SetAcceptedTime(slotTimeProvider.GenesisTime())

		t.Tangle.BlockDAG.ModelsTestFramework.SetBlock("Genesis", models.NewEmptyBlock(models.EmptyBlockID, models.WithIssuingTime(slotTimeProvider.GenesisTime())))
	}, (*TestFramework).createGenesis, (*TestFramework).setupEvents)
}

func (t *TestFramework) setupEvents() {
	t.Tangle.Instance.Events.Booker.VirtualVoting.BlockTracked.Hook(func(block *virtualvoting.Block) {
		if debug.GetEnabled() {
			t.test.Logf("SIMULATING SCHEDULED: %s", block.ID())
		}

		t.scheduledBlocksMutex.Lock()
		scheduledBlock := scheduler.NewBlock(block, scheduler.WithScheduled(true))
		t.scheduledBlocks.Set(block.ID(), scheduledBlock)
		t.scheduledBlocksMutex.Unlock()

		t.Instance.AddTip(scheduledBlock)
	})

	t.Engine.Events.EvictionState.SlotEvicted.Hook(func(index slot.Index) {
		t.Instance.EvictTSCCache(index)
	})

	t.Instance.Events.TipAdded.Hook(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP ADDED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipAdded), 1)
	})

	t.Instance.Events.TipRemoved.Hook(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP REMOVED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipRemoved), 1)
	})

	t.mockAcceptance.BlockAcceptedEvent.Hook(func(block *blockgadget.Block) {
		require.NoError(t.test, t.Engine.NotarizationManager.NotarizeAcceptedBlock(block.ModelsBlock))
	})
}

func (t *TestFramework) createGenesis() {
	genesisMarker := markers.NewMarker(0, 0)
	structureDetails := markers.NewStructureDetails()
	structureDetails.SetPastMarkers(markers.NewMarkers(genesisMarker))
	structureDetails.SetIsPastMarker(true)
	structureDetails.SetPastMarkerGap(0)

	block := scheduler.NewBlock(
		virtualvoting.NewBlock(
			blockdag.NewBlock(
				models.NewEmptyBlock(models.EmptyBlockID, models.WithIssuingTime(t.Tangle.Instance.BlockDAG.SlotTimeProvider.GenesisTime())),
				blockdag.WithSolid(true),
			),
			virtualvoting.WithBooked(true),
			virtualvoting.WithStructureDetails(structureDetails),
		),
		scheduler.WithScheduled(true),
	)

	t.scheduledBlocks.Set(block.ID(), block)

	t.SetBlocksAccepted("Genesis")
	t.SetMarkersAccepted(genesisMarker)
}

func (t *TestFramework) mockSchedulerBlock(id models.BlockID) (block *scheduler.Block, exists bool) {
	t.scheduledBlocksMutex.RLock()
	defer t.scheduledBlocksMutex.RUnlock()

	return t.scheduledBlocks.Get(id)
}

func (t *TestFramework) IssueBlocksAndSetAccepted(aliases ...string) {
	t.Tangle.BlockDAG.IssueBlocks(aliases...)
	t.SetBlocksAccepted(aliases...)
}

func (t *TestFramework) SetBlocksAccepted(aliases ...string) {
	for _, alias := range aliases {
		block := t.Tangle.Booker.Block(alias)
		t.mockAcceptance.SetBlockAccepted(blockgadget.NewBlock(block))
	}
}

func (t *TestFramework) SetMarkersAccepted(m ...markers.Marker) {
	t.mockAcceptance.SetMarkersAccepted(m...)
}

func (t *TestFramework) SetAcceptedTime(acceptedTime time.Time) {
	t.Engine.Clock.SetAcceptedTime(acceptedTime)
}

func (t *TestFramework) AssertIsPastConeTimestampCorrect(blockAlias string, expected bool) {
	block, exists := t.mockSchedulerBlock(t.Tangle.Booker.Block(blockAlias).ID())
	if !exists {
		panic(fmt.Sprintf("block with %s not found", blockAlias))
	}
	actual := t.Instance.IsPastConeTimestampCorrect(block.Block)
	require.Equal(t.test, expected, actual, "isPastConeTimestampCorrect: %s should be %t but is %t", blockAlias, expected, actual)
}

func (t *TestFramework) AssertTipsAdded(count uint32) {
	require.Equal(t.test, count, atomic.LoadUint32(&t.tipAdded), "expected %d tips to be added but got %d", count, atomic.LoadUint32(&t.tipAdded))
}

func (t *TestFramework) AssertTipsRemoved(count uint32) {
	require.Equal(t.test, count, atomic.LoadUint32(&t.tipRemoved), "expected %d tips to be removed but got %d", count, atomic.LoadUint32(&t.tipRemoved))
}

func (t *TestFramework) AssertEqualBlocks(actualBlocks, expectedBlocks models.BlockIDs) {
	require.Equal(t.test, expectedBlocks, actualBlocks, "expected blocks %s but got %s", expectedBlocks, actualBlocks)
}

func (t *TestFramework) AssertTips(expectedTips models.BlockIDs) {
	t.AssertEqualBlocks(models.NewBlockIDs(lo.Map(t.Instance.AllTips(), func(block *scheduler.Block) models.BlockID {
		return block.ID()
	})...), expectedTips)
}

func (t *TestFramework) AssertTipCount(expectedTipCount int) {
	require.Equal(t.test, expectedTipCount, t.Instance.TipCount(), "expected %d tip count but got %d", t.Instance.TipCount(), expectedTipCount)
}

func (t *TestFramework) FormCommitment(index slot.Index, acceptedBlocksAliases []string, prevIndex slot.Index) (cm *commitment.Commitment) {
	// acceptedBlocksInSlot := t.mockAcceptance.AcceptedBlocksInSlot(index)
	adsBlocks := ads.NewSet[models.BlockID](mapdb.NewMapDB())
	adsAttestations := ads.NewMap[identity.ID, notarization.Attestation](mapdb.NewMapDB())
	for _, acceptedBlockAlias := range acceptedBlocksAliases {
		acceptedBlock := t.Tangle.Booker.Block(acceptedBlockAlias)
		adsBlocks.Add(acceptedBlock.ID())
		adsAttestations.Set(acceptedBlock.IssuerID(), notarization.NewAttestation(acceptedBlock.ModelsBlock, t.Engine.SlotTimeProvider))
	}
	return commitment.New(
		index,
		lo.PanicOnErr(t.Engine.Storage.Commitments.Load(prevIndex)).ID(),
		commitment.NewRoots(
			adsBlocks.Root(),
			ads.NewSet[utxo.TransactionID](mapdb.NewMapDB()).Root(),
			adsAttestations.Root(),
			t.Engine.LedgerState.UnspentOutputs.Root(),
			ads.NewMap[identity.ID, sybilprotection.Weight](mapdb.NewMapDB()).Root(),
		).ID(),
		0,
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTipManagerOptions(opts ...options.Option[TipManager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTipManagerOptions = opts
	}
}

func WithEngineOptions(opts ...options.Option[engine.Engine]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsEngineOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
