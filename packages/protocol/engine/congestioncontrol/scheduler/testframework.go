package scheduler

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Scheduler *Scheduler

	test *testing.T

	scheduledBlocksCount uint32
	skippedBlocksCount   uint32
	droppedBlocksCount   uint32

	optsScheduler                   []options.Option[Scheduler]
	optsTangle                      []options.Option[tangle.Tangle]
	optsGadget                      []options.Option[acceptance.Gadget]
	optsValidatorSet                *validator.Set
	optsEvictionManager             *eviction.Manager[models.BlockID]
	optsAccessManaMapRetrieverFunc  func() map[identity.ID]float64
	optsTotalAccessManaRetrieveFunc func() float64
	optsRate                        time.Duration

	*TangleTestFramework
	*GadgetTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.optsEvictionManager == nil {
			t.optsEvictionManager = eviction.NewManager[models.BlockID](models.IsEmptyBlockID)
		}
		t.TangleTestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangle...),
			tangle.WithValidatorSet(t.optsValidatorSet),
			tangle.WithEvictionManager(t.optsEvictionManager),
		)
		t.GadgetTestFramework = acceptance.NewTestFramework(
			test,
			acceptance.WithGadgetOptions(t.optsGadget...),
			acceptance.WithValidatorSet(t.optsValidatorSet),
			acceptance.WithEvictionManager(t.optsEvictionManager),
			acceptance.WithTangleTestFramework(t.TangleTestFramework),
		)

		if t.Scheduler == nil {
			t.Scheduler = New(t.GadgetTestFramework.Gadget, t.TangleTestFramework.Tangle, t.optsAccessManaMapRetrieverFunc, t.optsTotalAccessManaRetrieveFunc, t.optsRate, t.optsScheduler...)
		}

	}, (*TestFramework).setupEvents)
}

type TangleTestFramework = tangle.TestFramework

type GadgetTestFramework = acceptance.TestFramework

func (t *TestFramework) setupEvents() {
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

func (t *TestFramework) CreteTangleBlock(opts ...options.Option[models.Block]) *virtualvoting.Block {
	blk := virtualvoting.NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(opts...))))
	if len(blk.ParentsByType(models.StrongParentType)) == 0 {
		parents := models.NewParentBlockIDs()
		parents.AddStrong(models.EmptyBlockID)
		opts = append(opts, models.WithParents(parents))
		blk = virtualvoting.NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(opts...))))

	}
	if err := blk.DetermineID(); err != nil {
		panic(errors.Wrap(err, "could not determine BlockID"))
	}

	return blk
}

func (t *TestFramework) AssertBlocksScheduled(blocksScheduled uint32) {
	assert.Equal(t.test, blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount), "expected %d blocks to be accepted but got %d", blocksScheduled, atomic.LoadUint32(&t.scheduledBlocksCount))
}

func (t *TestFramework) AssertBlocksSkipped(blocksSkipped uint32) {
	assert.Equal(t.test, blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount), "expected %d conflicts to be accepted but got %d", blocksSkipped, atomic.LoadUint32(&t.skippedBlocksCount))
}

func (t *TestFramework) AssertBlocksDropped(blocksDropped uint32) {
	assert.Equal(t.test, blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount), "expected %d conflicts to be rejected but got %d", blocksDropped, atomic.LoadUint32(&t.droppedBlocksCount))
}

func (t *TestFramework) ValidateScheduledBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.Scheduled()
		assert.Equal(t.test, expected, actual, "Block %s should be scheduled=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateSkippedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.Skipped()

		assert.Equal(t.test, expected, actual, "Block %s should be skipped=%t but is %t", blockID, expected, actual)
	}
}

func (t *TestFramework) ValidateDroppedBlocks(expectedState map[string]bool) {
	for blockID, expected := range expectedState {
		block, exists := t.Scheduler.Block(t.Block(blockID).ID())
		assert.Truef(t.test, exists, "block %s not registered", blockID)

		actual := block.Dropped()
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

func WithAccessManaMapRetrieverFunc(accessManaMapRetrieverFunc func() map[identity.ID]float64) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsAccessManaMapRetrieverFunc = accessManaMapRetrieverFunc
	}
}

func WithTotalAccessManaRetrieveFunc(totalAccessManaRetrieveFunc func() float64) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTotalAccessManaRetrieveFunc = totalAccessManaRetrieveFunc
	}
}

func WithRate(rate time.Duration) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsRate = rate
	}
}

func WithEvictionManager(evictionManager *eviction.Manager[models.BlockID]) options.Option[TestFramework] {
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
