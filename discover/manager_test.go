package discover

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wollac/autopeering/peer"
)

type NetworkMock struct {
	mock.Mock
}

func (m *NetworkMock) self() peer.ID {
	return [32]byte{}
}

func (m *NetworkMock) ping(p *peer.Peer) error {
	log.Println("ping")
	args := m.Called(p)
	return args.Error(0)
}

func (m *NetworkMock) requestPeers(p *peer.Peer) ([]*peer.Peer, error) {
	args := m.Called(p)
	return args.Get(0).([]*peer.Peer), args.Error(1)
}

func newDummyPeer(name string) *peer.Peer {
	return peer.NewPeer(peer.PublicKey([]byte(name)), name)
}

func newTestManager() (*manager, *NetworkMock, func()) {
	mock := new(NetworkMock)
	mgr := newManager(mock, nil, logger)
	teardown := func() {
		time.Sleep(graceTime)
		mgr.close()
	}
	return mgr, mock, teardown
}

func TestMgrClose(t *testing.T) {
	_, _, close := newTestManager()
	defer close()

	time.Sleep(graceTime)
}

func TestMgrVerifyDiscoveredPeer(t *testing.T) {
	mgr, m, close := newTestManager()
	defer close()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore requestPeers calls
	m.On("requestPeers", mock.Anything).Return([]*peer.Peer{}, nil).Maybe()

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addDiscoveredPeer(p)

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	m.AssertExpectations(t)
}

func TestMgrReverifyPeer(t *testing.T) {
	mgr, m, close := newTestManager()
	defer close()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore requestPeers calls
	m.On("requestPeers", mock.Anything).Return([]*peer.Peer{}, nil).Maybe()

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	m.AssertExpectations(t)
}

func TestMgrRequestDiscoveredPeer(t *testing.T) {
	mgr, m, close := newTestManager()
	defer close()

	p1 := newDummyPeer("verified")
	p2 := newDummyPeer("discovered")

	// expect requestPeers on the discovered peer
	m.On("requestPeers", p1).Return([]*peer.Peer{p2}, nil).Maybe()
	// ignore any ping
	m.On("ping", mock.Anything).Return(nil).Maybe()

	mgr.addVerifiedPeer(p1)
	mgr.addDiscoveredPeer(p2)

	time.Sleep(graceTime)
	m.AssertExpectations(t)
}

func TestMgrAddManyVerifiedPeers(t *testing.T) {
	mgr, m, close := newTestManager()
	defer close()

	p := newDummyPeer("p")

	// expect ping of peer p
	m.On("ping", p).Return(nil).Once()
	// ignore requestPeers calls
	m.On("requestPeers", mock.Anything).Return([]*peer.Peer{}, nil)

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)
	for i := 0; i < maxKnow+maxReplacements; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := mgr.GetVerifiedPeers()

	assert.Equal(t, maxKnow, len(ps))
	assert.Contains(t, ps, p)
}

func TestMgrDeleteUnreachablePeer(t *testing.T) {
	mgr, m, close := newTestManager()
	defer close()

	p := newDummyPeer("p")

	// expect ping of peer p, but return error
	m.On("ping", p).Return(errTimeout).Times(reverifyTries)
	// ignore requestPeers calls
	m.On("requestPeers", mock.Anything).Return([]*peer.Peer{}, nil)

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)
	for i := 0; i < maxKnow; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := mgr.GetVerifiedPeers()

	assert.Equal(t, maxKnow, len(ps))
	assert.NotContains(t, ps, p)
}
