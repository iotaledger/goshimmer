package ratesetter

import (
	"fmt"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/utils"
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
		func() {
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
		}()
	}
}

func TestRateSetter_NoSchedulerCongestion(t *testing.T) {
	enabledModes := []ModeType{AIMDMode, DeficitMode}
	numBlocks := 2 * utils.MaxLocalQueueSize

	for _, mode := range enabledModes {
		func() {
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
		}()
	}
}

func TestRateSetter_Rate(t *testing.T) {
	rate := 50 * time.Millisecond // 20 tx/s -> 1s total processing time for the full queue
	numBlocks := 2 * utils.MaxLocalQueueSize
	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DeficitMode), WithSchedulerRate(rate)), WithSchedulerOptions(scheduler.WithRate(rate), scheduler.WithMaxDeficit(300)))
	defer tf.RateSetter.Shutdown()

	blockIssued := make(chan *models.Block, numBlocks)
	tf.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) { blockIssued <- block }))
	tf.SubmitBlocks(numBlocks)
	startTime := time.Now()
	assert.LessOrEqualf(t, tf.RateSetter.Size(), utils.MaxLocalQueueSize, "RateSetter queue size is greated than maximum allowed %d", tf.RateSetter.Size())

	totalIssued := 0
	for range blockIssued {
		totalIssued++
		if tf.RateSetter.Size() == 0 {
			totalTime := time.Since(startTime)
			avgRate := float64(totalIssued) / totalTime.Seconds()
			assert.InEpsilon(t, 20, avgRate, .2)
			break
		}
	}
}

func TestRateSetter_SchedulerEstimate(t *testing.T) {
	rate := 100 * time.Millisecond // 10 tx/s -> 2s total processing time for the full queue
	numBlocks := 2 * utils.MaxLocalQueueSize

	tf := NewTestFramework(t, WithRateSetterOptions(WithMode(DeficitMode), WithSchedulerRate(rate)), WithSchedulerOptions(scheduler.WithRate(rate), scheduler.WithMaxDeficit(300)))
	defer tf.RateSetter.Shutdown()

	blockIssued := make(chan *models.Block, numBlocks)
	tf.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) { blockIssued <- block }))
	fmt.Println(tf.RateSetter.Estimate())

	// make rate setter queue full

	maxSubmitted := 2 * utils.MaxLocalQueueSize
	submitted := 0
	for tf.RateSetter.Size() < utils.MaxLocalQueueSize/2 {
		tf.SubmitBlocks(1)
		submitted++
		if submitted > maxSubmitted {
			break
		}
	}
	fmt.Println(tf.RateSetter.Size())
	fmt.Println(tf.RateSetter.Estimate())
	time.Sleep(tf.RateSetter.Estimate())

	lastBlock := tf.CreateBlock()
	t0 := time.Now()
	assert.NoError(tf.test, tf.RateSetter.SubmitBlock(lastBlock))

	assert.Eventually(t, func() bool {
		select {
		case blk := <-blockIssued:
			if blk.ID() == lastBlock.ID() {
				timeElapsed := time.Now().Sub(t0)
				fmt.Println(timeElapsed)
				return true
			}
		default:
			return false
		}
		return false
	}, 10*time.Second, time.Microsecond)

	// await all queue is fully processed
	assert.Eventually(t, func() bool {
		return tf.RateSetter.Size() == 0
	}, 3*time.Second, 10*time.Millisecond)

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
