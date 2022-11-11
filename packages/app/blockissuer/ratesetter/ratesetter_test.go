package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
)

func TestRateSetter_StartStop(t *testing.T) {
	localID := identity.GenerateIdentity().ID()
	allModes := []RateSetterModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, localID, mode)
		defer tf.RateSetter.Shutdown()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRateSetter_IssueBlock(t *testing.T) {
	localIdentity := identity.GenerateIdentity()
	localID := localIdentity.ID()
	allModes := []RateSetterModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, localID, mode)

		defer tf.RateSetter.Shutdown()

		blockIssued := make(chan *models.Block, 1)
		tf.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) { blockIssued <- block }))

		blk := models.NewBlock(models.WithIssuer(localIdentity.PublicKey()))
		assert.NoError(t, tf.RateSetter.IssueBlock(blk))
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

func TestRateSetter_WebAPI(t *testing.T) {
	localID := identity.GenerateIdentity().ID()
	allModes := []RateSetterModeType{AIMDMode, DeficitMode, DisabledMode}

	for _, mode := range allModes {
		tf := NewTestFramework(t, localID, mode)
		tf.RateSetter.Rate()
		tf.RateSetter.Estimate()
		tf.RateSetter.Size()
	}
}
