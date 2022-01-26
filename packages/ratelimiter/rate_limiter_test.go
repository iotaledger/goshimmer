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
	testCount(t, prl, defaultTestLimit)
}

func TestPeerRateLimiter_SetLimit(t *testing.T) {
	t.Parallel()
	prl := newTestRateLimiter(t)
	customLimit := 5
	prl.SetLimit(customLimit)
	testCount(t, prl, customLimit)
}

func testCount(t testing.TB, prl *ratelimiter.PeerRateLimiter, testLimit int) {
	expectedPeer := newTestPeer()
	activityCount := atomic.NewInt32(0)
	expectedActivity := testLimit + 1
	eventCalled := atomic.NewInt32(0)
	prl.HitEvent().Attach(events.NewClosure(func(p *peer.Peer, rl *ratelimiter.RateLimit) {
		eventCalled.Inc()
		assert.Equal(t, int32(expectedActivity), activityCount.Load())
		assert.Equal(t, expectedPeer, p)
		assert.Equal(t, defaultTestInterval, rl.Interval)
		assert.Equal(t, testLimit, rl.Limit)
	}))
	for i := 0; i < expectedActivity; i++ {
		activityCount.Inc()
		prl.Count(expectedPeer)
	}
	assert.Eventually(t, func() bool { return eventCalled.Load() == 1 }, time.Second, time.Millisecond)
	for i := 0; i < expectedActivity; i++ {
		activityCount.Inc()
		prl.Count(expectedPeer)
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
