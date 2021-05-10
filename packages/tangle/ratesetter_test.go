package tangle

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

var (
	testBeta             = 0.7
	testInitial          = 20000.0
	testRateSetterParams = RateSetterParams{
		Beta:    &testBeta,
		Initial: &testInitial,
	}
)

func TestRateSetter_StartStop(t *testing.T) {
	localID := identity.GenerateLocalIdentity()

	tangle := newTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()
	time.Sleep(10 * time.Millisecond)
}

func TestRateSetter_Submit(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := newTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()

	msg := newMessage(localNode.PublicKey())
	tangle.RateSetter.Submit(msg)
	time.Sleep(100 * time.Millisecond)
}

func TestRateSetter_ErrorHandling(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	peerID := identity.GenerateLocalIdentity()
	peerNode := identity.New(peerID.PublicKey())

	tangle := newTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()

	// Test 1: non-local node issuer message is not accepted
	{
		var otherMsg int32
		tangle.Events.Info.Attach(events.NewClosure(func(str string) { atomic.AddInt32(&otherMsg, 1) }))

		// message issued by other nodes
		msg1 := newMessage(peerNode.PublicKey())
		tangle.RateSetter.Submit(msg1)

		msg2 := newMessage(localNode.PublicKey())
		tangle.RateSetter.Submit(msg2)

		assert.Eventually(t, func() bool {
			return assert.Equal(t, int32(1), otherMsg)
		}, 1*time.Second, 10*time.Millisecond)
	}

	// Test 2: Discard messages if exceeding issuingQueue size
	{
		messageDiscarded := make(chan MessageID, 1)
		discardedCounter := events.NewClosure(func(id MessageID) { messageDiscarded <- id })
		tangle.RateSetter.Events.MessageDiscarded.Attach(discardedCounter)

		// set issuingQueue size to maximum
		queueSize := tangle.RateSetter.issuingQueue.Size()
		tangle.RateSetter.issuingQueue.SetSize(uint(MaxLocalQueueSize))

		msg := newMessage(localNode.PublicKey())
		tangle.RateSetter.Submit(msg)

		assert.Eventually(t, func() bool {
			select {
			case id := <-messageDiscarded:
				return assert.Equal(t, msg.ID(), id)
			default:
				return false
			}
		}, 1*time.Second, 10*time.Millisecond)

		tangle.RateSetter.Events.MessageDiscarded.Detach(discardedCounter)
		tangle.RateSetter.issuingQueue.SetSize(queueSize)
	}
}
