package ratesetter

import (
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_SubmitBlock(t *testing.T) {
	allModes := []ModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode)))

		defer tf.RateSetter.Shutdown()

		blockIssued := make(chan *models.Block, 1)
		tf.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) { blockIssued <- block }))
		blk := tf.CreateBlock()

		assert.NoError(t, tf.RateSetter.SubmitBlock(blk))
		assert.Eventually(t, func() bool {
			select {
			case blk1 := <-blockIssued:
				return assert.Equal(t, blk, blk1)
			default:
				return false
			}
		}, 1*time.Second, 10*time.Millisecond)
	}
}

func TestRateSetter_NoSchedulerCongestion(t *testing.T) {
	enabledModes := []ModeType{DeficitMode}
	numBlocks := 100

	for _, mode := range enabledModes {
		rate := 10 * time.Millisecond
		tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode), WithSchedulerRate(rate)), WithSchedulerOptions(scheduler.WithRate(rate), scheduler.WithMaxDeficit(300)))
		defer tf.RateSetter.Shutdown()

		blockIssued := make(chan *models.Block, numBlocks)
		tf.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) { blockIssued <- block }))
		tf.SubmitBlocks(numBlocks)
		for range blockIssued {
			assert.Less(t, tf.Scheduler.BufferSize(), 5)
			//fmt.Printf("Block issued with size %d. %d blocks in the Issuer queue. %d blocks in the Scheduler queue.\n", blk.Size(), tf.RateSetter.Size(), tf.Scheduler.BufferSize())
			if tf.RateSetter.Size() == 0 {
				break
			}
		}
	}
}

func TestRateSetter_SchedulerEstimate(t *testing.T) {
	rate := 10 * time.Millisecond
	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DeficitMode), WithSchedulerRate(rate)), WithSchedulerOptions(scheduler.WithRate(rate), scheduler.WithMaxDeficit(300)))
	defer tf.RateSetter.Shutdown()

}

func TestRateSetter_WebAPI(t *testing.T) {
	allModes := []ModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode)))
		tf.RateSetter.Rate()
		tf.RateSetter.Estimate()
		tf.RateSetter.Size()
	}
}
