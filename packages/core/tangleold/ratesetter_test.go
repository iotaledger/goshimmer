package tangleold

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
)

var (
	testInitial          = 5.0
	testRateSetterParams = RateSetterParams{
		Initial:          testInitial,
		RateSettingPause: time.Second,
		Enabled:          true,
	}
)

func TestRateSetter_StartStop(t *testing.T) {
	localID := identity.GenerateLocalIdentity()

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()
	time.Sleep(10 * time.Millisecond)
}

func TestRateSetter_Submit(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()
	rateSetter := NewRateSetter(tangle)
	defer rateSetter.Shutdown()

	blockIssued := make(chan *Block, 1)
	rateSetter.Events.BlockIssued.Attach(event.NewClosure(func(event *BlockConstructedEvent) { blockIssued <- event.Block }))

	blk := newBlock(localNode.PublicKey())
	assert.NoError(t, rateSetter.Issue(blk))
	assert.Eventually(t, func() bool {
		select {
		case blk1 := <-blockIssued:
			return assert.Equal(t, blk, blk1)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestRateSetter_ErrorHandling(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()
	rateSetter := NewRateSetter(tangle)
	defer rateSetter.Shutdown()

	blockDiscarded := make(chan BlockID, MaxLocalQueueSize*2)
	discardedCounter := event.NewClosure(func(event *BlockDiscardedEvent) { blockDiscarded <- event.BlockID })
	rateSetter.Events.BlockDiscarded.Hook(discardedCounter)
	for i := 0; i < MaxLocalQueueSize*2; i++ {
		blk := NewBlock(
			emptyLikeReferencesFromStrongParents(NewBlockIDs(EmptyBlockID)),
			time.Now(),
			localNode.PublicKey(),
			0,
			payload.NewGenericDataPayload(make([]byte, MaxLocalQueueSize)),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
		)
		assert.NoError(t, blk.DetermineID())
		assert.NoError(t, rateSetter.Issue(blk))
	}

	assert.Eventually(t, func() bool {
		select {
		case <-blockDiscarded:
			return true
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}
