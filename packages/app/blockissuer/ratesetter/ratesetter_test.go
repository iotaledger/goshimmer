package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_SubmitBlock(t *testing.T) {
	allModes := []RateSetterModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterMode(mode))

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
	enabledModes := []RateSetterModeType{AIMDMode, DeficitMode}
	numBlocks := 100

	for _, mode := range enabledModes {
		tf := NewTestFramework(t, WithRateSetterMode(mode), WithSchedulerRate(10*time.Millisecond), WithMaxDeficit(300))
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

func TestRateSetter_WebAPI(t *testing.T) {
	allModes := []RateSetterModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, WithRateSetterMode(mode))
		tf.RateSetter.Rate()
		tf.RateSetter.Estimate()
		tf.RateSetter.Size()
	}
}
