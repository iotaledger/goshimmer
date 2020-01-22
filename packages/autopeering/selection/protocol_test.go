package selection

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graceTime = 100 * time.Millisecond

var log = logger.NewExampleLogger("selection")

var peerMap = make(map[peer.ID]*peer.Peer)

// dummyDiscovery is a dummy implementation of DiscoveryProtocol never returning any verified peers.
type dummyDiscovery struct{}

func (d dummyDiscovery) IsVerified(peer.ID, string) bool                 { return true }
func (d dummyDiscovery) EnsureVerified(*peer.Peer)                       {}
func (d dummyDiscovery) GetVerifiedPeer(id peer.ID, _ string) *peer.Peer { return peerMap[id] }
func (d dummyDiscovery) GetVerifiedPeers() []*peer.Peer                  { return []*peer.Peer{} }

// newTest creates a new neighborhood server and also returns the teardown.
func newTest(t require.TestingT, trans transport.Transport) (*server.Server, *Protocol, func()) {
	l := log.Named(trans.LocalAddr().String())
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), db)
	require.NoError(t, err)

	// add the new peer to the global map for dummyDiscovery
	peerMap[local.ID()] = &local.Peer

	prot := New(local, dummyDiscovery{}, Config{Log: l})
	srv := server.Listen(local, trans, l.Named("srv"), prot)
	prot.Start(srv)

	teardown := func() {
		srv.Close()
		prot.Close()
		db.Close()
	}
	return srv, prot, teardown
}

func getPeer(s *server.Server) *peer.Peer {
	return &s.Local().Peer
}

func TestProtocol(t *testing.T) {
	// assure that the default test parameters are used for all protocol tests
	SetParameters(Parameters{
		SaltLifetime:           testSaltLifetime,
		OutboundUpdateInterval: testUpdateInterval,
	})

	t.Run("PeeringRequest", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		srvA, protA, closeA := newTest(t, p2p.A)
		defer closeA()
		srvB, protB, closeB := newTest(t, p2p.B)
		defer closeB()

		peerA := getPeer(srvA)
		saltA, _ := salt.NewSalt(100 * time.Second)
		peerB := getPeer(srvB)
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

		_, protA, closeA := newTest(t, p2p.A)
		defer closeA()
		srvB, _, closeB := newTest(t, p2p.B)
		defer closeB()

		saltA, _ := salt.NewSalt(-1 * time.Second)
		peerB := getPeer(srvB)

		// request peering to peer B
		_, err := protA.PeeringRequest(peerB, saltA)
		assert.EqualError(t, err, server.ErrTimeout.Error())
	})

	t.Run("PeeringDrop", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		srvA, protA, closeA := newTest(t, p2p.A)
		defer closeA()
		srvB, protB, closeB := newTest(t, p2p.B)
		defer closeB()

		peerA := getPeer(srvA)
		saltA, _ := salt.NewSalt(100 * time.Second)
		peerB := getPeer(srvB)

		// request peering to peer B
		services, err := protA.PeeringRequest(peerB, saltA)
		require.NoError(t, err)
		assert.NotEmpty(t, services)

		require.Contains(t, protB.GetNeighbors(), peerA)

		// drop peer A
		protA.PeeringDrop(peerB)
		time.Sleep(graceTime)
		require.NotContains(t, protB.GetNeighbors(), peerA)
	})

	t.Run("FullTest", func(t *testing.T) {
		p2p := transport.P2P()
		defer p2p.Close()

		srvA, protA, closeA := newFullTest(t, p2p.A)
		defer closeA()

		time.Sleep(graceTime) // wait for the master to initialize

		srvB, protB, closeB := newFullTest(t, p2p.B, getPeer(srvA))
		defer closeB()

		time.Sleep(outboundUpdateInterval + graceTime) // wait for the next outbound cycle

		// the two peers should be peered
		assert.ElementsMatch(t, []*peer.Peer{getPeer(srvB)}, protA.GetNeighbors())
		assert.ElementsMatch(t, []*peer.Peer{getPeer(srvA)}, protB.GetNeighbors())
	})
}

// newTest creates a new server handling discover as well as neighborhood and also returns the teardown.
func newFullTest(t require.TestingT, trans transport.Transport, masterPeers ...*peer.Peer) (*server.Server, *Protocol, func()) {
	l := log.Named(trans.LocalAddr().String())
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), db)
	require.NoError(t, err)

	discovery := discover.New(local, discover.Config{
		Log:         l.Named("disc"),
		MasterPeers: masterPeers,
	})
	selection := New(local, discovery, Config{
		Log: l.Named("sel"),
	})

	srv := server.Listen(local, trans, l.Named("srv"), discovery, selection)

	discovery.Start(srv)
	selection.Start(srv)

	teardown := func() {
		srv.Close()
		selection.Close()
		discovery.Close()
		db.Close()
	}
	return srv, selection, teardown
}
