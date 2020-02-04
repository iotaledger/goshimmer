package selection

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/peertest"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/iotaledger/goshimmer/packages/database/mapdb"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNetwork = "udp"
	graceTime   = 100 * time.Millisecond
)

var (
	log     = logger.NewExampleLogger("discover")
	peerMap = make(map[peer.ID]*peer.Peer)
)

func TestProtocol(t *testing.T) {
	// assure that the default test parameters are used for all protocol tests
	SetParameters(Parameters{
		SaltLifetime:           testSaltLifetime,
		OutboundUpdateInterval: testUpdateInterval,
	})

	t.Run("PeeringRequest", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		protA, closeA := newTestProtocol(p2p.A)
		defer closeA()
		protB, closeB := newTestProtocol(p2p.B)
		defer closeB()

		peerA := getPeer(protA)
		saltA, _ := salt.NewSalt(100 * time.Second)
		peerB := getPeer(protB)
		saltB, _ := salt.NewSalt(100 * time.Second)

		// request peering to peer B
		t.Run("A->B", func(t *testing.T) {
			if services, err := protA.PeeringRequest(peerB, saltA); assert.NoError(t, err) {
				assert.NotEmpty(t, services)
			}
		})
		// request peering to peer A
		t.Run("B->A", func(t *testing.T) {
			if services, err := protB.PeeringRequest(peerA, saltB); assert.NoError(t, err) {
				assert.NotEmpty(t, services)
			}
		})
	})

	t.Run("ExpiredSalt", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		protA, closeA := newTestProtocol(p2p.A)
		defer closeA()
		protB, closeB := newTestProtocol(p2p.B)
		defer closeB()

		saltA, _ := salt.NewSalt(-1 * time.Second)
		peerB := getPeer(protB)

		// request peering to peer B
		_, err := protA.PeeringRequest(peerB, saltA)
		assert.EqualError(t, err, server.ErrTimeout.Error())
	})

	t.Run("PeeringDrop", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		protA, closeA := newTestProtocol(p2p.A)
		defer closeA()
		protB, closeB := newTestProtocol(p2p.B)
		defer closeB()

		peerA := getPeer(protA)
		saltA, _ := salt.NewSalt(100 * time.Second)
		peerB := getPeer(protB)

		// request peering to peer B
		status, err := protA.PeeringRequest(peerB, saltA)
		require.NoError(t, err)
		assert.True(t, status)

		require.Contains(t, protB.GetNeighbors(), peerA)

		// drop peer A
		protA.PeeringDrop(peerB)
		time.Sleep(graceTime)
		require.NotContains(t, protB.GetNeighbors(), peerA)
	})

	t.Run("FullTest", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		protA, closeA := newFullTestProtocol(p2p.A)
		defer closeA()

		time.Sleep(graceTime) // wait for the master to initialize

		protB, closeB := newFullTestProtocol(p2p.B, getPeer(protA))
		defer closeB()

		time.Sleep(outboundUpdateInterval + graceTime) // wait for the next outbound cycle

		// the two peers should be peered
		assert.ElementsMatch(t, []*peer.Peer{getPeer(protB)}, protA.GetNeighbors())
		assert.ElementsMatch(t, []*peer.Peer{getPeer(protA)}, protB.GetNeighbors())
	})
}

// dummyDiscovery is a dummy implementation of DiscoveryProtocol never returning any verified peers.
type dummyDiscovery struct{}

func (d dummyDiscovery) IsVerified(peer.ID, string) bool                 { return true }
func (d dummyDiscovery) EnsureVerified(*peer.Peer) error                 { return nil }
func (d dummyDiscovery) GetVerifiedPeer(id peer.ID, _ string) *peer.Peer { return peerMap[id] }
func (d dummyDiscovery) GetVerifiedPeers() []*peer.Peer                  { return []*peer.Peer{} }

// newTestProtocol creates a new neighborhood server and also returns the teardown.
func newTestProtocol(trans transport.Transport) (*Protocol, func()) {
	db, _ := peer.NewDB(mapdb.NewMapDB())
	local := peertest.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), db)
	// add the new peer to the global map for dummyDiscovery
	peerMap[local.ID()] = &local.Peer
	l := log.Named(trans.LocalAddr().String())

	prot := New(local, dummyDiscovery{}, Config{Log: l.Named("disc")})
	srv := server.Serve(local, trans, l.Named("srv"), prot)
	prot.Start(srv)

	teardown := func() {
		srv.Close()
		prot.Close()
	}
	return prot, teardown
}

// newTestProtocol creates a new server handling discover as well as neighborhood and also returns the teardown.
func newFullTestProtocol(trans transport.Transport, masterPeers ...*peer.Peer) (*Protocol, func()) {
	db, _ := peer.NewDB(mapdb.NewMapDB())
	local := peertest.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), db)
	// add the new peer to the global map for dummyDiscovery
	peerMap[local.ID()] = &local.Peer
	l := log.Named(trans.LocalAddr().String())

	discovery := discover.New(local, discover.Config{
		Log:         l.Named("disc"),
		MasterPeers: masterPeers,
	})
	selection := New(local, discovery, Config{
		Log: l.Named("sel"),
	})

	srv := server.Serve(local, trans, l.Named("srv"), discovery, selection)

	discovery.Start(srv)
	selection.Start(srv)

	teardown := func() {
		srv.Close()
		selection.Close()
		discovery.Close()
	}
	return selection, teardown
}

func getPeer(p *Protocol) *peer.Peer {
	return &p.local().Peer
}
