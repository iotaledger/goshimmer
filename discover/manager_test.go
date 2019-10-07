package discover

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/server"
)

type NetworkMock struct {
	mock.Mock

	loc *peer.Local
}

func (m *NetworkMock) local() *peer.Local {
	return m.loc
}

func (m *NetworkMock) ping(p *peer.Peer) error {
	log.Println("ping")
	args := m.Called(p)
	return args.Error(0)
}

func (m *NetworkMock) discoveryRequest(p *peer.Peer) ([]*peer.Peer, error) {
	args := m.Called(p)
	return args.Get(0).([]*peer.Peer), args.Error(1)
}

func newNetworkMock() *NetworkMock {
	priv, _ := peer.GeneratePrivateKey()
	return &NetworkMock{
		// no database needed
		loc: peer.NewLocal(priv, nil),
	}
}

func newDummyPeer(name string) *peer.Peer {
	return peer.NewPeer([]byte(name), name)
}

func newTestManager() (*manager, *NetworkMock, func()) {
	networkMock := newNetworkMock()
	mgr := newManager(networkMock, nil, logger)
	teardown := func() {
		time.Sleep(graceTime)
		mgr.close()
	}
	return mgr, networkMock, teardown
}

func TestMgrClose(t *testing.T) {
	_, _, teardown := newTestManager()
	defer teardown()

	time.Sleep(graceTime)
}

func TestMgrVerifyDiscoveredPeer(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore discoveryRequest calls
	m.On("discoveryRequest", mock.Anything).Return([]*peer.Peer{}, nil).Maybe()

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addDiscoveredPeer(p)

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	m.AssertExpectations(t)
}

func TestMgrReverifyPeer(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore discoveryRequest calls
	m.On("discoveryRequest", mock.Anything).Return([]*peer.Peer{}, nil).Maybe()

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	m.AssertExpectations(t)
}

func TestMgrRequestDiscoveredPeer(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p1 := newDummyPeer("verified")
	p2 := newDummyPeer("discovered")

	// expect discoveryRequest on the discovered peer
	m.On("discoveryRequest", p1).Return([]*peer.Peer{p2}, nil).Maybe()
	// ignore any ping
	m.On("ping", mock.Anything).Return(nil).Maybe()

	mgr.addVerifiedPeer(p1)
	mgr.addDiscoveredPeer(p2)

	time.Sleep(graceTime)
	m.AssertExpectations(t)
}

func TestMgrAddManyVerifiedPeers(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore discoveryRequest calls
	m.On("discoveryRequest", mock.Anything).Return([]*peer.Peer{}, nil)

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)
	for i := 0; i < maxKnow+maxReplacements; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := unwrapPeers(mgr.getVerifiedPeers())

	assert.Equal(t, maxKnow, len(ps))
	assert.Contains(t, ps, p)
}

func TestMgrDeleteUnreachablePeer(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p := newDummyPeer("p")

	// expect ping of peer p, but return error
	m.On("ping", p).Return(server.ErrTimeout).Times(reverifyTries)
	// ignore discoveryRequest calls
	m.On("discoveryRequest", mock.Anything).Return([]*peer.Peer{}, nil)

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)
	for i := 0; i < maxKnow; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := unwrapPeers(mgr.getVerifiedPeers())

	assert.Equal(t, maxKnow, len(ps))
	assert.NotContains(t, ps, p)
}
