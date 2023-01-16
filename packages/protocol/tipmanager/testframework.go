package tipmanager

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
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
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	TipManager           *TipManager
	engine               *engine.Engine
	mockAcceptance       *blockgadget.MockAcceptanceGadget
	scheduledBlocks      *shrinkingmap.ShrinkingMap[models.BlockID, *scheduler.Block]
	scheduledBlocksMutex sync.RWMutex

	test       *testing.T
	tipAdded   uint32
	tipRemoved uint32

	optsTipManagerOptions   []options.Option[TipManager]
	optsTangleOptions       []options.Option[tangle.Tangle]
	optsNotarizationOptions []options.Option[notarization.Manager]
	*tangle.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:                    test,
		mockAcceptance:          blockgadget.NewMockAcceptanceGadget(),
		scheduledBlocks:         shrinkingmap.New[models.BlockID, *scheduler.Block](),
		optsNotarizationOptions: []options.Option[notarization.Manager]{notarization.MinCommittableEpochAge(time.Since(time.Unix(epoch.GenesisTime, 0)))},
	}, opts, func(t *TestFramework) {
		storageInstance := storage.New(test.TempDir(), 1)
		test.Cleanup(func() {
			event.Loop.PendingTasksCounter.WaitIsZero()
			t.engine.Shutdown()
			storageInstance.Shutdown()
		})

		// set MinCommittableEpochAge to genesis so nothing is committed.
		t.engine = engine.New(storageInstance, dpos.NewProvider(), mana1.NewProvider(), engine.WithNotarizationManagerOptions(t.optsNotarizationOptions...), engine.WithTangleOptions(t.optsTangleOptions...))

		t.TestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangle(t.engine.Tangle),
			tangle.WithLedger(t.engine.Ledger),
			tangle.WithEvictionState(t.engine.EvictionState),
			tangle.WithValidators(t.engine.SybilProtection.Validators()),
		)

		if t.TipManager == nil {
			t.TipManager = New(t.mockSchedulerBlock, t.optsTipManagerOptions...)
			// TODO: need to activate it with an engine t.TipManager.Activate()
		}

		t.TipManager.LinkTo(t.engine)

		t.TipManager.blockAcceptanceGadget = t.mockAcceptance

		t.SetAcceptedTime(time.Unix(epoch.GenesisTime, 0))

		t.TestFramework.ModelsTestFramework.SetBlock("Genesis", models.NewEmptyBlock(models.EmptyBlockID, models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0))))
	}, (*TestFramework).setupEvents, (*TestFramework).createGenesis)
}

func (t *TestFramework) setupEvents() {
	t.Tangle.Events.VirtualVoting.BlockTracked.Hook(event.NewClosure(func(block *virtualvoting.Block) {
		if debug.GetEnabled() {
			t.test.Logf("SIMULATING SCHEDULED: %s", block.ID())
		}

		t.scheduledBlocksMutex.Lock()
		scheduledBlock := scheduler.NewBlock(block, scheduler.WithScheduled(true))
		t.scheduledBlocks.Set(block.ID(), scheduledBlock)
		t.scheduledBlocksMutex.Unlock()

		t.TipManager.AddTip(scheduledBlock)
	}))

	t.TipManager.Events.TipAdded.Hook(event.NewClosure(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP ADDED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipAdded), 1)
	}))

	t.TipManager.Events.TipRemoved.Hook(event.NewClosure(func(block *scheduler.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TIP REMOVED: %s", block.ID())
		}
		atomic.AddUint32(&(t.tipRemoved), 1)
	}))

	t.mockAcceptance.BlockAcceptedEvent.Attach(event.NewClosure(func(block *blockgadget.Block) {
		t.engine.NotarizationManager.NotarizeAcceptedBlock(block.ModelsBlock)
	}))

	t.engine.NotarizationManager.Events.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		t.TipManager.PromoteFutureTips(details.Commitment)
	}))

	t.engine.EvictionState.Events.EpochEvicted.Attach(event.NewClosure(func(index epoch.Index) {
		t.TipManager.Evict(index)
	}))
}

func (t *TestFramework) createGenesis() {
	genesisMarker := markers.NewMarker(0, 0)
	structureDetails := markers.NewStructureDetails()
	structureDetails.SetPastMarkers(markers.NewMarkers(genesisMarker))
	structureDetails.SetIsPastMarker(true)
	structureDetails.SetPastMarkerGap(0)

	block := scheduler.NewBlock(
		virtualvoting.NewBlock(
			booker.NewBlock(
				blockdag.NewBlock(
					models.NewEmptyBlock(models.EmptyBlockID, models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0))),
					blockdag.WithSolid(true),
				),
				booker.WithBooked(true),
				booker.WithStructureDetails(structureDetails),
			),
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

// WaitUntilAllTasksProcessed waits until all tasks are processed.
func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	for _, wp := range t.engine.WorkerPools() {
		wp.PendingTasksCounter.WaitIsZero()
	}
	event.Loop.PendingTasksCounter.WaitIsZero()

	return t
}

func (t *TestFramework) IssueBlocksAndSetAccepted(aliases ...string) *blockdag.TestFramework {
	t.SetBlocksAccepted(aliases...)

	return t.IssueBlocks(aliases...)
}

func (t *TestFramework) SetBlocksAccepted(aliases ...string) {
	for _, alias := range aliases {
		block := t.Block(alias)
		t.mockAcceptance.SetBlockAccepted(blockgadget.NewBlock(virtualvoting.NewBlock(block)))
	}
}

func (t *TestFramework) SetMarkersAccepted(m ...markers.Marker) {
	t.mockAcceptance.SetMarkersAccepted(m...)
}

func (t *TestFramework) SetAcceptedTime(acceptedTime time.Time) {
	t.engine.Clock.SetAcceptedTime(acceptedTime)
}

func (t *TestFramework) AssertIsPastConeTimestampCorrect(blockAlias string, expected bool) {
	block, exists := t.mockSchedulerBlock(t.Block(blockAlias).ID())
	if !exists {
		panic(fmt.Sprintf("block with %s not found", blockAlias))
	}
	actual := t.TipManager.isPastConeTimestampCorrect(block.Block.Block)
	assert.Equal(t.test, expected, actual, "isPastConeTimestampCorrect: %s should be %t but is %t", blockAlias, expected, actual)
}

func (t *TestFramework) AssertTipsAdded(count uint32) {
	assert.Equal(t.test, count, atomic.LoadUint32(&t.tipAdded), "expected %d tips to be added but got %d", count, atomic.LoadUint32(&t.tipAdded))
}

func (t *TestFramework) AssertTipsRemoved(count uint32) {
	assert.Equal(t.test, count, atomic.LoadUint32(&t.tipRemoved), "expected %d tips to be removed but got %d", count, atomic.LoadUint32(&t.tipRemoved))
}

func (t *TestFramework) AssertEqualBlocks(actualBlocks, expectedBlocks models.BlockIDs) {
	require.Equal(t.test, expectedBlocks, actualBlocks, "expected blocks %s but got %s", expectedBlocks, actualBlocks)
}

func (t *TestFramework) AssertTips(expectedTips models.BlockIDs) {
	t.AssertEqualBlocks(models.NewBlockIDs(lo.Map(t.TipManager.AllTips(), func(block *scheduler.Block) models.BlockID {
		return block.ID()
	})...), expectedTips)
}

func (t *TestFramework) AssertFutureTips(expectedFutureTips map[epoch.Index]map[commitment.ID]models.BlockIDs) {
	actualFutureTips := make(map[epoch.Index]map[commitment.ID]models.BlockIDs)

	t.TipManager.futureTips.ForEach(func(index epoch.Index, commitmentStorage *memstorage.Storage[commitment.ID, *memstorage.Storage[models.BlockID, *scheduler.Block]]) {
		commitmentStorage.ForEach(func(cm commitment.ID, tipStorage *memstorage.Storage[models.BlockID, *scheduler.Block]) bool {
			if _, exists := actualFutureTips[index]; !exists {
				actualFutureTips[index] = make(map[commitment.ID]models.BlockIDs)
			}

			if _, exists := actualFutureTips[index][cm]; !exists {
				actualFutureTips[index][cm] = models.NewBlockIDs()
			}

			tipStorage.ForEach(func(blockID models.BlockID, _ *scheduler.Block) bool {
				actualFutureTips[index][cm].Add(blockID)
				return true
			})

			return true
		})
	})

	require.Equal(t.test, expectedFutureTips, actualFutureTips, "expected future tips %s but got %s", expectedFutureTips, actualFutureTips)
}

func (t *TestFramework) AssertTipCount(expectedTipCount int) {
	assert.Equal(t.test, expectedTipCount, t.TipManager.TipCount(), "expected %d tip count but got %d", t.TipManager.TipCount(), expectedTipCount)
}

func (t *TestFramework) FormCommitment(index epoch.Index, acceptedBlocksAliases []string, prevIndex epoch.Index) (cm *commitment.Commitment) {
	// acceptedBlocksInEpoch := t.mockAcceptance.AcceptedBlocksInEpoch(index)
	adsBlocks := ads.NewSet[models.BlockID](mapdb.NewMapDB())
	adsAttestations := ads.NewMap[identity.ID, notarization.Attestation](mapdb.NewMapDB())
	for _, acceptedBlockAlias := range acceptedBlocksAliases {
		acceptedBlock := t.Block(acceptedBlockAlias)
		adsBlocks.Add(acceptedBlock.ID())
		adsAttestations.Set(acceptedBlock.IssuerID(), notarization.NewAttestation(acceptedBlock.ModelsBlock))
	}
	return commitment.New(
		index,
		lo.PanicOnErr(t.engine.Storage.Commitments.Load(prevIndex)).ID(),
		commitment.NewRoots(
			adsBlocks.Root(),
			ads.NewSet[utxo.TransactionID](mapdb.NewMapDB()).Root(),
			adsAttestations.Root(),
			t.engine.LedgerState.UnspentOutputs.Root(),
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

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangleOptions = opts
	}
}

func WithNotarizationOptions(opts ...options.Option[notarization.Manager]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsNotarizationOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
