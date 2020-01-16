package discover

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type NetworkMock struct {
	mock.Mock

	loc *peer.Local
}

func (m *NetworkMock) local() *peer.Local {
	return m.loc
}

func (m *NetworkMock) ping(p *peer.Peer) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *NetworkMock) discoveryRequest(p *peer.Peer) ([]*peer.Peer, error) {
	args := m.Called(p)
	return args.Get(0).([]*peer.Peer), args.Error(1)
}

func newNetworkMock() *NetworkMock {
	local, _ := peer.NewLocal("mock", "0", peer.NewMemoryDB(log))
	return &NetworkMock{
		// no database needed
		loc: local,
	}
}

func newDummyPeer(name string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, "dummy", name)

	return peer.NewPeer([]byte(name), services)
}

func newTestManager() (*manager, *NetworkMock, func()) {
	networkMock := newNetworkMock()
	mgr := newManager(networkMock, nil, log)
	teardown := func() {
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
	m.On("discoveryRequest", p1).Return([]*peer.Peer{p2}, nil).Once()
	// ignore any ping
	m.On("ping", mock.Anything).Return(nil).Maybe()

	mgr.addVerifiedPeer(p1)
	mgr.addDiscoveredPeer(p2)

	mgr.doQuery(make(chan time.Duration, 1)) // manually trigger a query
	m.AssertExpectations(t)
}

func TestMgrAddManyVerifiedPeers(t *testing.T) {
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
	for i := 0; i < maxManaged+maxReplacements; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := unwrapPeers(mgr.getVerifiedPeers())

	assert.Equal(t, maxManaged, len(ps))
	assert.Contains(t, ps, p)

	m.AssertExpectations(t)
}

func TestMgrDeleteUnreachablePeer(t *testing.T) {
	mgr, m, teardown := newTestManager()
	defer teardown()

	p := newDummyPeer("p")

	// expect ping of peer p, but return error
	m.On("ping", p).Return(server.ErrTimeout).Times(reverifyTries)
	// ignore discoveryRequest calls
	m.On("discoveryRequest", mock.Anything).Return([]*peer.Peer{}, nil).Maybe()

	// let the manager initialize
	time.Sleep(graceTime)

	mgr.addVerifiedPeer(p)
	for i := 0; i < maxManaged; i++ {
		mgr.addVerifiedPeer(newDummyPeer(fmt.Sprintf("p%d", i)))
	}

	mgr.doReverify(make(chan struct{})) // manually trigger a verify
	ps := unwrapPeers(mgr.getVerifiedPeers())

	assert.Equal(t, maxManaged, len(ps))
	assert.NotContains(t, ps, p)

	m.AssertExpectations(t)
}
