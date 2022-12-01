package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"

	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_IssueBlock(t *testing.T) {
	allModes := []ModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		func() {
			tf := NewTestFramework(t, WithRateSetterOptions(WithMode(mode)))

			defer tf.RateSetter.Shutdown()

			blockScheduled := make(chan *models.Block, 1)
			tf.Protocol.CongestionControl.Scheduler().Events.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) { blockScheduled <- block.ModelsBlock }))
			blk := tf.CreateBlock()

			assert.NoError(t, tf.IssueBlock(blk))
			assert.Eventually(t, func() bool {
				select {
				case blk1 := <-blockScheduled:
					return assert.Equal(t, blk, blk1)
				default:
					return false
				}
			}, 1*time.Second, 10*time.Millisecond)
		}()
	}
}
