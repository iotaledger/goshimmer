package ratelimiter_test

import (
	"net"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/ratelimiter"
)

const (
	defaultTestInterval = time.Minute
	defaultTestLimit    = 3
)

func TestPeerRateLimiter_Count(t *testing.T) {
	t.Parallel()
	prl := newTestRateLimiter(t)
	testPeer := newTestPeer()
	testCount(t, prl, testPeer, defaultTestLimit)
}

func TestPeerRateLimiter_SetBaseLimit(t *testing.T) {
	t.Parallel()
	prl := newTestRateLimiter(t)
	customLimit := 5
	prl.SetBaseLimit(customLimit)
	testPeer := newTestPeer()
	testCount(t, prl, testPeer, customLimit)
}

func TestPeerRateLimiter_ExtendLimit(t *testing.T) {
	t.Parallel()
	prl := newTestRateLimiter(t)
	testPeer := newTestPeer()
	limitExtensionCount := 3
	for i := 0; i < limitExtensionCount; i++ {
		prl.ExtendLimit(testPeer, 1)
	}
	testCount(t, prl, testPeer, defaultTestLimit+limitExtensionCount)
}

func testCount(t testing.TB, prl *ratelimiter.PeerRateLimiter, testPeer *peer.Peer, testLimit int) {
	activityCount := atomic.NewInt32(0)
	expectedActivity := testLimit + 1
	eventCalled := atomic.NewInt32(0)
	prl.HitEvent().Attach(events.NewClosure(func(p *peer.Peer, rl *ratelimiter.RateLimit) {
		eventCalled.Inc()
		assert.Equal(t, int32(expectedActivity), activityCount.Load())
		assert.Equal(t, testPeer, p)
		assert.Equal(t, defaultTestInterval, rl.Interval)
		assert.Equal(t, testLimit, rl.Limit)
	}))
	for i := 0; i < expectedActivity; i++ {
		activityCount.Inc()
		prl.Count(testPeer)
	}
	assert.Eventually(t, func() bool { return eventCalled.Load() == 1 }, time.Second, time.Millisecond)
	for i := 0; i < expectedActivity; i++ {
		activityCount.Inc()
		prl.Count(testPeer)
	}
	assert.Never(t, func() bool { return eventCalled.Load() > 1 }, time.Second, time.Millisecond)
}

func newTestRateLimiter(t testing.TB) *ratelimiter.PeerRateLimiter {
	prl, err := ratelimiter.NewPeerRateLimiter(defaultTestInterval, defaultTestLimit, logger.NewNopLogger())
	require.NoError(t, err)
	return prl
}

func newTestPeer() *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, "tcp", 0)
	services.Update(service.GossipKey, "tcp", 0)

	var publicKey ed25519.PublicKey
	copy(publicKey[:], "test peer")

	return peer.NewPeer(identity.New(publicKey), net.IPv4zero, services)
}
