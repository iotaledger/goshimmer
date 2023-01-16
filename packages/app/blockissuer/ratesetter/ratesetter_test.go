package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"

	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_IssueBlockAndAwaitSchedule_AIMD(t *testing.T) {

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(AIMDMode)))
	defer tf.Shutdown()

	blockScheduled := make(chan *models.Block, 1)
	tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))
	blk := tf.CreateBlock(0)

	assert.NoError(t, tf.IssueBlock(blk, 0))
	assert.Eventually(t, func() bool {
		select {
		case blk1 := <-blockScheduled:
			return assert.Equal(t, blk, blk1)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

}

func TestRateSetter_IssueBlockAndAwaitSchedule_Deficit(t *testing.T) {

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DeficitMode)))
	defer tf.Shutdown()

	blockScheduled := make(chan *models.Block, 1)
	tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))
	blk := tf.CreateBlock(0)

	assert.NoError(t, tf.IssueBlock(blk, 0))
	assert.Eventually(t, func() bool {
		select {
		case blk1 := <-blockScheduled:
			return assert.Equal(t, blk, blk1)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

}

func TestRateSetter_IssueBlockAndAwaitSchedule_Disabled(t *testing.T) {

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DisabledMode)))
	defer tf.Shutdown()

	blockScheduled := make(chan *models.Block, 1)
	tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))
	blk := tf.CreateBlock(0)

	assert.NoError(t, tf.IssueBlock(blk, 0))
	assert.Eventually(t, func() bool {
		select {
		case blk1 := <-blockScheduled:
			return assert.Equal(t, blk, blk1)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

}

func TestRateSetter_IssueBlocksAndAwaitScheduleMultipleIssuers_Deficit(t *testing.T) {
	numIssuers := 5
	numBlocksPerIssuer := 10
	schedulerRate := time.Duration(100) * time.Nanosecond // 100 nanoseconds between scheduling each block.

	allBlocks := make(map[models.BlockID]*models.Block)

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DeficitMode)), WithSchedulerOptions(scheduler.WithRate(schedulerRate)), WithNumIssuers(numIssuers))
	defer tf.Shutdown()
	blockScheduled := make(chan *models.Block, 1)
	tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))

	for i := 0; i < numIssuers; i++ {
		blocks := tf.IssueBlocks(numBlocksPerIssuer, i)
		for blkID, blk := range blocks {
			allBlocks[blkID] = blk
		}
	}

	assert.Eventually(t, func() bool {
		select {
		case blk := <-blockScheduled:
			delete(allBlocks, blk.ID())
			return len(allBlocks) == 0
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

}

func TestRateSetter_IssueBlocksAndAwaitScheduleMultipleIssuers_Disabled(t *testing.T) {
	numIssuers := 5
	numBlocksPerIssuer := 10
	schedulerRate := time.Duration(100) * time.Nanosecond // 100 nanoseconds between scheduling each block.

	allBlocks := make(map[models.BlockID]*models.Block)

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DisabledMode)), WithSchedulerOptions(scheduler.WithRate(schedulerRate)), WithNumIssuers(numIssuers))
	defer tf.Shutdown()
	blockScheduled := make(chan *models.Block, 1)
	tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))

	for i := 0; i < numIssuers; i++ {
		blocks := tf.IssueBlocks(numBlocksPerIssuer, i)
		for blkID, blk := range blocks {
			allBlocks[blkID] = blk
		}
	}

	assert.Eventually(t, func() bool {
		select {
		case blk := <-blockScheduled:
			delete(allBlocks, blk.ID())
			return len(allBlocks) == 0
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)

}
