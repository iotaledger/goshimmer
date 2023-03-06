package scheduler

import (
	"sync/atomic"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Scheduler *Scheduler
	Tangle    *tangle.TestFramework
	workers   *workerpool.Group

	storage        *storage.Storage
	engine         *engine.Engine
	mockAcceptance *blockgadget.MockAcceptanceGadget
	issuersByAlias map[string]*identity.Identity
	issuersMana    map[identity.ID]int64

	test *testing.T

	scheduledBlocksCount uint32
	skippedBlocksCount   uint32
	droppedBlocksCount   uint32
	evictionState        *eviction.State
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, slotTimeProvider *slot.TimeProvider, optsScheduler ...options.Option[Scheduler]) *TestFramework {
	t := &TestFramework{
		test:           test,
		workers:        workers,
		issuersMana:    make(map[identity.ID]int64),
		issuersByAlias: make(map[string]*identity.Identity),
		mockAcceptance: blockgadget.NewMockAcceptanceGadget(),
	}
	t.storage = storage.New(test.TempDir(), 1)

	t.engine = engine.New(workers.CreateGroup("Engine"), t.storage, dpos.NewProvider(), mana1.NewProvider(), slotTimeProvider)
	test.Cleanup(func() {
		t.Scheduler.Shutdown()
		t.engine.Shutdown()
		workers.WaitChildren()
		t.storage.Shutdown()
	})

	t.Tangle = tangle.NewTestFramework(
		test,
		t.engine.Tangle,
		booker.NewTestFramework(test, workers.CreateGroup("BookerTestFramework"), t.engine.Tangle.Booker),
	)

	t.Scheduler = New(t.Tangle.BlockDAG.Instance.EvictionState, slotTimeProvider, t.mockAcceptance.IsBlockAccepted, t.ManaMap, t.TotalMana, optsScheduler...)

	t.setupEvents()

	return t
}

func (t *TestFramework) setupEvents() {
	t.mockAcceptance.BlockAcceptedEvent.Hook(t.Scheduler.HandleAcceptedBlock, event.WithWorkerPool(t.workers.CreatePool("HandleAccepted", 2)))
	t.Tangle.Instance.Events.Booker.VirtualVoting.BlockTracked.Hook(t.Scheduler.AddBlock, event.WithWorkerPool(t.workers.CreatePool("Add", 2)))
	t.Tangle.Instance.Events.BlockDAG.BlockOrphaned.Hook(t.Scheduler.HandleOrphanedBlock, event.WithWorkerPool(t.workers.CreatePool("HandleOrphaned", 2)))

	t.Scheduler.Events.BlockScheduled.Hook(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("SCHEDULED: %s", block.ID())
		}

		atomic.AddUint32(&(t.scheduledBlocksCount), 1)
	})

	t.Scheduler.Events.BlockSkipped.Hook(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK SKIPPED: %s", block.ID())
		}
		atomic.AddUint32(&(t.skippedBlocksCount), 1)
	})

	t.Scheduler.Events.BlockDropped.Hook(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK DROPPED: %s", block.ID())
		}
		atomic.AddUint32(&(t.droppedBlocksCount), 1)
	})
}

func (t *TestFramework) CreateIssuer(alias string, issuerMana int64) {
	t.issuersByAlias[alias] = identity.GenerateIdentity()
	t.issuersMana[t.issuersByAlias[alias].ID()] = issuerMana
}

func (t *TestFramework) UpdateIssuers(newIssuers map[string]int64) {
	for alias, mana := range newIssuers {
		_, exists := t.issuersByAlias[alias]
		if !exists {
			t.issuersByAlias[alias] = identity.GenerateIdentity()
		}
		t.issuersMana[t.issuersByAlias[alias].ID()] = mana
	}

	for alias, issuerIdentity := range t.issuersByAlias {
		_, exists := newIssuers[alias]
		if !exists {
			delete(t.issuersMana, issuerIdentity.ID())
		}
	}
}

func (t *TestFramework) Issuer(alias string) (issuerIdentity *identity.Identity) {
	issuerIdentity, exists := t.issuersByAlias[alias]
	if !exists {
		panic("identity alias not registered")
	}
	return issuerIdentity
}

func (t *TestFramework) CreateSchedulerBlock(opts ...options.Option[models.Block]) *Block {
	blk := virtualvoting.NewBlock(blockdag.NewBlock(models.NewBlock(opts...), blockdag.WithSolid(true)), virtualvoting.WithBooked(true), virtualvoting.WithStructureDetails(markers.NewStructureDetails()))
	if len(blk.ParentsByType(models.StrongParentType)) == 0 {
		parents := models.NewParentBlockIDs()
		parents.AddStrong(models.EmptyBlockID)
		opts = append(opts, models.WithParents(parents))
		blk = virtualvoting.NewBlock(blockdag.NewBlock(models.NewBlock(opts...), blockdag.WithSolid(true)), virtualvoting.WithBooked(true), virtualvoting.WithStructureDetails(markers.NewStructureDetails()))
	}
	if err := blk.DetermineID(t.Tangle.Instance.BlockDAG.SlotTimeProvider); err != nil {
		panic(errors.Wrap(err, "could not determine BlockID"))
	}

	schedulerBlock, _ := t.Scheduler.GetOrRegisterBlock(blk)

	return schedulerBlock
}

func (t *TestFramework) TotalMana() (totalMana int64) {
	for _, mana := range t.issuersMana {
		totalMana += mana
	}
	return
}

func (t *TestFramework) ManaMap() map[identity.ID]int64 {
	return t.issuersMana
}

func (t *TestFramework) AssertBlocksScheduled(blocksScheduled uint32) {
	require.Equal(t.test, blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount), "expected %d blocks to be scheduled but got %d", blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount))
}

func (t *TestFramework) AssertBlocksSkipped(blocksSkipped uint32) {
	require.Equal(t.test, blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount), "expected %d blocks to be skipped but got %d", blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount))
}

func (t *TestFramework) AssertBlocksDropped(blocksDropped uint32) {
	require.Equal(t.test, blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount), "expected %d blocks to be dropped but got %d", blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount))
}

func (t *TestFramework) ValidateScheduledBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Tangle.BlockDAG.Block(blockID).ID())
		require.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsScheduled()
		require.Equal(t.test, expected, actual, "Block %s should be scheduled=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateSkippedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Tangle.BlockDAG.Block(blockID).ID())
		require.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsSkipped()

		require.Equal(t.test, expected, actual, "Block %s should be skipped=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateDroppedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Tangle.BlockDAG.Block(blockID).ID())
		require.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsDropped()
		require.Equal(t.test, expected, actual, "Block %s should be dropped=%t but is %t", blockID, expected, actual)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
