package scheduler

import (
	"sync/atomic"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Scheduler      *Scheduler
	mockAcceptance *acceptance.MockAcceptanceGadget
	issuersByAlias map[string]*identity.Identity
	issuersMana    map[identity.ID]int64

	test *testing.T

	scheduledBlocksCount uint32
	skippedBlocksCount   uint32
	droppedBlocksCount   uint32

	optsScheduler           []options.Option[Scheduler]
	optsTangle              []options.Option[tangle.Tangle]
	optsGadget              []options.Option[acceptance.Gadget]
	optsValidatorSet        *validator.Set
	optsEvictionManager     *eviction.State[models.BlockID]
	optsIsBlockAcceptedFunc func(models.BlockID) bool
	optsBlockAcceptedEvent  *event.Linkable[*acceptance.Block]
	*TangleTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test:           test,
		issuersMana:    make(map[identity.ID]int64),
		issuersByAlias: make(map[string]*identity.Identity),
		mockAcceptance: acceptance.NewMockAcceptanceGadget(),
	}, opts, func(t *TestFramework) {
		if t.optsEvictionManager == nil {
			t.optsEvictionManager = eviction.NewState[models.BlockID]()
		}
		if t.optsValidatorSet == nil {
			t.optsValidatorSet = validator.NewSet()
		}

		t.TangleTestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangle...),
			tangle.WithValidatorSet(t.optsValidatorSet),
			tangle.WithEvictionManager(t.optsEvictionManager),
		)

		if t.optsIsBlockAcceptedFunc == nil {
			t.optsIsBlockAcceptedFunc = t.mockAcceptance.IsBlockAccepted
		}
		if t.optsBlockAcceptedEvent == nil {
			t.optsBlockAcceptedEvent = t.mockAcceptance.BlockAcceptedEvent
		}

		if t.Scheduler == nil {
			t.Scheduler = New(t.TangleTestFramework.BlockDAG.EvictionManager.State, t.optsIsBlockAcceptedFunc, t.ManaMap, t.TotalMana, t.optsScheduler...)
		}

	}, (*TestFramework).setupEvents)
}

type TangleTestFramework = tangle.TestFramework

type GadgetTestFramework = acceptance.TestFramework

func (t *TestFramework) setupEvents() {
	t.mockAcceptance.BlockAcceptedEvent.Attach(event.NewClosure(t.Scheduler.HandleAcceptedBlock))
	t.Tangle.Events.VirtualVoting.BlockTracked.Attach(event.NewClosure(t.Scheduler.AddBlock))
	t.Tangle.Events.BlockDAG.BlockOrphaned.Attach(event.NewClosure(t.Scheduler.HandleOrphanedBlock))

	t.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("SCHEDULED: %s", block.ID())
		}

		atomic.AddUint32(&(t.scheduledBlocksCount), 1)
	}))

	t.Scheduler.Events.BlockSkipped.Hook(event.NewClosure(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK SKIPPED: %s", block.ID())
		}
		atomic.AddUint32(&(t.skippedBlocksCount), 1)
	}))

	t.Scheduler.Events.BlockDropped.Hook(event.NewClosure(func(block *Block) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK DROPPED: %s", block.ID())
		}
		atomic.AddUint32(&(t.droppedBlocksCount), 1)
	}))

	return
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
	blk := virtualvoting.NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(opts...), blockdag.WithSolid(true)), booker.WithBooked(true), booker.WithStructureDetails(markers.NewStructureDetails())))
	if len(blk.ParentsByType(models.StrongParentType)) == 0 {
		parents := models.NewParentBlockIDs()
		parents.AddStrong(models.EmptyBlockID)
		opts = append(opts, models.WithParents(parents))
		blk = virtualvoting.NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(opts...), blockdag.WithSolid(true)), booker.WithBooked(true), booker.WithStructureDetails(markers.NewStructureDetails())))
	}
	if err := blk.DetermineID(); err != nil {
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
	assert.Equal(t.test, blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount), "expected %d blocks to be scheduled but got %d", blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount))
}

func (t *TestFramework) AssertBlocksSkipped(blocksSkipped uint32) {
	assert.Equal(t.test, blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount), "expected %d blocks to be skipped but got %d", blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount))
}

func (t *TestFramework) AssertBlocksDropped(blocksDropped uint32) {
	assert.Equal(t.test, blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount), "expected %d blocks to be dropped but got %d", blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount))
}

func (t *TestFramework) ValidateScheduledBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsScheduled()
		assert.Equal(t.test, expected, actual, "Block %s should be scheduled=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateSkippedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsSkipped()

		assert.Equal(t.test, expected, actual, "Block %s should be skipped=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateDroppedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.IsDropped()
		assert.Equal(t.test, expected, actual, "Block %s should be dropped=%t but is %t", blockID, expected, actual)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSchedulerOptions(opts ...options.Option[Scheduler]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsScheduler = opts
	}
}

func WithGadgetOptions(opts ...options.Option[acceptance.Gadget]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsGadget = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = opts
	}
}

func WithBlockAcceptedEvent(blockAcceptedEvent *event.Linkable[*acceptance.Block]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsBlockAcceptedEvent = blockAcceptedEvent
	}
}
func WithIsBlockAcceptedFunc(isBlockAcceptedFunc func(id models.BlockID) bool) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsIsBlockAcceptedFunc = isBlockAcceptedFunc
	}
}

func WithEvictionManager(evictionManager *eviction.State[models.BlockID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionManager = evictionManager
	}
}

func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidatorSet = validatorSet
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
