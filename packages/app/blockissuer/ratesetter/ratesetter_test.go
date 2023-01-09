package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"

	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_IssueBlockAndAwaitSchedule(t *testing.T) {
	allModes := []ModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode)))

		defer tf.RateSetter[0].Shutdown()

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
}

func TestRateSetter_IssueBlocksAndAwaitScheduleMultipleIssuers(t *testing.T) {
	allModes := []ModeType{DeficitMode, DisabledMode}
	numIssuers := 5
	numBlocksPerIssuer := 10
	schedulerRate := 100

	allBlocks := make(map[models.BlockID]*models.Block)

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode)), WithSchedulerOptions(scheduler.WithRate(time.Duration(schedulerRate))), WithNumIssuers(numIssuers))
		blockScheduled := make(chan *models.Block, 1)
		tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))

		for i := 0; i < numIssuers; i++ {
			defer tf.RateSetter[i].Shutdown()
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
}
