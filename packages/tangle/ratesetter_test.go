package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

var (
	testInitial          = 20000.0
	testRateSetterParams = RateSetterParams{
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
	rateSetter := NewRateSetter(tangle)
	defer rateSetter.Shutdown()

	messageRated := make(chan *Message, 1)
	rateSetter.Events.MessageRated.Attach(events.NewClosure(func(msg *Message) { messageRated <- msg }))

	msg := newMessage(localNode.PublicKey())
	assert.NoError(t, rateSetter.Issue(msg))
	assert.Eventually(t, func() bool {
		select {
		case msg1 := <-messageRated:
			return assert.Equal(t, msg, msg1)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestRateSetter_ErrorHandling(t *testing.T) {
	localID := identity.GenerateLocalIdentity()
	localNode := identity.New(localID.PublicKey())

	tangle := newTestTangle(Identity(localID), RateSetterConfig(testRateSetterParams))
	defer tangle.Shutdown()
	rateSetter := NewRateSetter(tangle)
	defer rateSetter.Shutdown()

	messageDiscarded := make(chan MessageID, 1)
	discardedCounter := events.NewClosure(func(id MessageID) { messageDiscarded <- id })
	rateSetter.Events.MessageDiscarded.Attach(discardedCounter)

	msg := NewMessage(
		[]MessageID{EmptyMessageID},
		[]MessageID{},
		time.Now(),
		localNode.PublicKey(),
		0,
		payload.NewGenericDataPayload(make([]byte, MaxLocalQueueSize)),
		0,
		ed25519.Signature{},
	)
	assert.NoError(t, rateSetter.Issue(msg))

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageDiscarded:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}
