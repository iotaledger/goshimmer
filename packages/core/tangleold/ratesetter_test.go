package tangleold

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

var (
	testInitial          = 5.0
	testRateSetterParams = RateSetterParams{
		Initial:          testInitial,
		RateSettingPause: time.Second,
		Mode:             AIMDMode,
	}
	testInitialDef          = 5.0
	testRateSetterParamsDef = RateSetterParams{
		Initial:          testInitialDef,
		RateSettingPause: time.Second,
		Mode:             DeficitMode,
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
	rateSetter := NewRateSetter(tangle).(*AIMDRateSetter)
	defer rateSetter.Shutdown()

	blockIssued := make(chan *Block, 1)
	rateSetter.RateSetterEvents().BlockIssued.Attach(event.NewClosure(func(event *BlockConstructedEvent) { blockIssued <- event.Block }))

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
	rateSetter := NewRateSetter(tangle).(*AIMDRateSetter)
	defer rateSetter.Shutdown()

	blockDiscarded := make(chan BlockID, MaxLocalQueueSize*2)
	discardedCounter := event.NewClosure(func(event *BlockDiscardedEvent) { blockDiscarded <- event.BlockID })
	rateSetter.RateSetterEvents().BlockDiscarded.Hook(discardedCounter)
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

func TestRateSetterDef_StartStop(t *testing.T) {
	localID := identity.GenerateLocalIdentity()

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParamsDef))
	defer tangle.Shutdown()
	time.Sleep(10 * time.Millisecond)
}

func TestRateSetterDef_Submit(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParamsDef))
	defer tangle.Shutdown()
	rateSetter := NewRateSetter(tangle).(*DeficitRateSetter)
	defer rateSetter.Shutdown()

	blockIssued := make(chan *Block, 1)
	rateSetter.RateSetterEvents().BlockIssued.Attach(event.NewClosure(func(event *BlockConstructedEvent) { blockIssued <- event.Block }))

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

func TestRateSetterDef_ErrorHandling(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := NewTestTangle(Identity(localID), RateSetterConfig(testRateSetterParamsDef))
	defer tangle.Shutdown()
	rateSetter := NewRateSetter(tangle).(*DeficitRateSetter)
	defer rateSetter.Shutdown()

	blockDiscarded := make(chan BlockID, MaxLocalQueueSize*2)
	discardedCounter := event.NewClosure(func(event *BlockDiscardedEvent) { blockDiscarded <- event.BlockID })
	rateSetter.RateSetterEvents().BlockDiscarded.Hook(discardedCounter)

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
	// no check for discarded because everything will be issued immediately by deficit-based ratesetter rather than added to issuing queue.
}
