package selection

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/salt"
	"github.com/iotaledger/autopeering-sim/server"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const graceTime = 100 * time.Millisecond

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}

// dummyDiscovery is a dummy implementation of DiscoveryProtocol never returning any verified peers.
type dummyDiscovery struct{}

func (d dummyDiscovery) IsVerified(p *peer.Peer) bool   { return true }
func (d dummyDiscovery) EnsureVerified(p *peer.Peer)    {}
func (d dummyDiscovery) GetVerifiedPeers() []*peer.Peer { return []*peer.Peer{} }

// newTest creates a new neighborhood server and also returns the teardown.
func newTest(t require.TestingT, name string, trans transport.Transport, logger *zap.SugaredLogger) (*server.Server, *Protocol, func()) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal(db)
	require.NoError(t, err)

	// add a dummy service
	local.Services()["dummy"] = peer.NetworkAddress{}

	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)

	cfg := Config{
		Log:          log,
		SaltLifetime: DefaultSaltLifetime,
	}
	prot := New(local, dummyDiscovery{}, cfg)
	srv := server.Listen(local, trans, log.Named("srv"), prot)
	prot.Start(srv)

	teardown := func() {
		srv.Close()
		prot.Close()
		db.Close()
	}
	return srv, prot, teardown
}

func TestProtPeeringRequest(t *testing.T) {
	p2p := transport.P2P()

	srvA, protA, closeA := newTest(t, "A", p2p.A, logger)
	defer closeA()
	srvB, protB, closeB := newTest(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	saltA, _ := salt.NewSalt(100 * time.Second)
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())
	saltB, _ := salt.NewSalt(100 * time.Second)

	// request peering to peer B
	t.Run("A->B", func(t *testing.T) {
		if services, err := protA.RequestPeering(peerB, saltA, srvA.Local().Services()); assert.NoError(t, err) {
			assert.NotEmpty(t, services)
		}
	})
	// request peering to peer A
	t.Run("B->A", func(t *testing.T) {
		if services, err := protB.RequestPeering(peerA, saltB, srvB.Local().Services()); assert.NoError(t, err) {
			assert.NotEmpty(t, services)
		}
	})
}

func TestProtExpiredSalt(t *testing.T) {
	p2p := transport.P2P()

	srvA, protA, closeA := newTest(t, "A", p2p.A, logger)
	defer closeA()
	srvB, _, closeB := newTest(t, "B", p2p.B, logger)
	defer closeB()

	saltA, _ := salt.NewSalt(-1 * time.Second)
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peering to peer B
	_, err := protA.RequestPeering(peerB, saltA, srvA.Local().Services())
	assert.EqualError(t, err, server.ErrTimeout.Error())
}

func TestProtDropPeer(t *testing.T) {
	p2p := transport.P2P()

	srvA, protA, closeA := newTest(t, "A", p2p.A, logger)
	defer closeA()
	srvB, protB, closeB := newTest(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	saltA, _ := salt.NewSalt(100 * time.Second)
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peering to peer B
	services, err := protA.RequestPeering(peerB, saltA, srvA.Local().Services())
	require.NoError(t, err)
	assert.NotEmpty(t, services)

	require.Contains(t, protB.GetNeighbors(), peerA)

	// drop peer A
	protA.DropPeer(peerB)
	time.Sleep(graceTime)
	require.NotContains(t, protB.GetNeighbors(), peerA)
}
