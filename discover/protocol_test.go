package discover

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

// newTestServer creates a new discovery server and also returns the teardown.
func newTestServer(t require.TestingT, name string, trans transport.Transport, logger *zap.SugaredLogger, masters ...*peer.Peer) (*server.Server, *Protocol, func()) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal(db)
	require.NoError(t, err)

	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)

	cfg := Config{
		Log:         log,
		MasterPeers: masters,
	}
	prot := New(local, cfg)
	srv := server.Listen(local, trans, log.Named("srv"), prot)
	prot.Start(srv)

	teardown := func() {
		srv.Close()
		prot.Close()
		db.Close()
	}
	return srv, prot, teardown
}

func TestProtVerifyMaster(t *testing.T) {
	p2p := transport.P2P()

	srvA, _, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())

	// use peerA as masters peer
	_, protB, closeB := newTestServer(t, "B", p2p.B, logger, peerA)

	time.Sleep(graceTime) // wait for the packages to ripple through the network
	closeB()              // close srvB to avoid race conditions, when asserting

	if assert.EqualValues(t, 1, len(protB.mgr.known)) {
		assert.EqualValues(t, peerA, &protB.mgr.known[0].Peer)
		assert.EqualValues(t, 1, protB.mgr.known[0].verifiedCount)
	}
}

func TestProtPingPong(t *testing.T) {
	p2p := transport.P2P()

	srvA, protA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, protB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send a ping from node A to B
	t.Run("A->B", func(t *testing.T) { assert.NoError(t, protA.ping(peerB)) })
	time.Sleep(graceTime)

	// send a ping from node B to A
	t.Run("B->A", func(t *testing.T) { assert.NoError(t, protB.ping(peerA)) })
	time.Sleep(graceTime)
}

func TestProtPingTimeout(t *testing.T) {
	p2p := transport.P2P()

	_, protA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, _, closeB := newTestServer(t, "B", p2p.B, logger)
	closeB() // close the connection right away to prevent any replies

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send a ping from node A to B
	err := protA.ping(peerB)
	assert.EqualError(t, err, server.ErrTimeout.Error())
}

func TestProtDiscoveryRequest(t *testing.T) {
	p2p := transport.P2P()

	srvA, protA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, protB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peers from node A
	t.Run("A->B", func(t *testing.T) {
		if ps, err := protA.discoveryRequest(peerB); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerA}, ps)
		}
	})
	// request peers from node B
	t.Run("B->A", func(t *testing.T) {
		if ps, err := protB.discoveryRequest(peerA); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerB}, ps)
		}
	})
}

func BenchmarkPingPong(b *testing.B) {
	p2p := transport.P2P()
	log := zap.NewNop().Sugar() // disable logging

	_, protA, closeA := newTestServer(b, "A", p2p.A, log)
	defer closeA()
	srvB, _, closeB := newTestServer(b, "B", p2p.B, log)
	defer closeB()

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		// send a ping from node A to B
		_ = protA.ping(peerB)
	}

	b.StopTimer()
}

func BenchmarkDiscoveryRequest(b *testing.B) {
	p2p := transport.P2P()
	log := zap.NewNop().Sugar() // disable logging

	_, protA, closeA := newTestServer(b, "A", p2p.A, log)
	defer closeA()
	srvB, _, closeB := newTestServer(b, "B", p2p.B, log)
	defer closeB()

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send initial request to ensure that every peer is verified
	_, err := protA.discoveryRequest(peerB)
	require.NoError(b, err)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = protA.discoveryRequest(peerB)
	}

	b.StopTimer()
}
