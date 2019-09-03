package discover

import (
	"log"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wollac/autopeering/peer"
	pb "github.com/wollac/autopeering/proto"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

const graceTime = 5 * time.Millisecond

var (
	testAddr = "127.0.0.1:8888"
	testPing = &pb.Ping{Version: 0, From: testAddr, To: testAddr}
)

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}

func assertProto(t *testing.T, got, want proto.Message) {
	if !proto.Equal(got, want) {
		t.Errorf("got %v want %v\n", got, want)
	}
}

// newTestServer creates a new discovery server and also returns the teardown.
func newTestServer(t require.TestingT, name string, trans transport.Transport, logger *zap.SugaredLogger, boot ...*peer.Peer) (*Server, func()) {
	priv, err := peer.GeneratePrivateKey()
	require.NoError(t, err)

	log := logger.Named(name)
	db := peer.NewMapDB(log.Named("db"))
	local := peer.NewLocal(priv, db)

	cfg := Config{
		Log:       logger.Named(name),
		Bootnodes: boot,
	}
	srv := Listen(trans, local, cfg)

	teardown := func() {
		time.Sleep(graceTime) // wait a short time for all the packages to propagate
		srv.Close()
		db.Close()
	}
	return srv, teardown
}

func TestSrvEncodeDecodePing(t *testing.T) {
	priv, err := peer.GeneratePrivateKey()
	require.NoError(t, err)
	// create minimal server just containing the private key
	s := &Server{local: peer.NewLocal(priv, nil)}

	ping := testPing
	packet := s.encode(ping)

	wrapper, key, err := decode(packet)
	require.NoError(t, err)

	assert.EqualValues(t, priv.Public(), key)
	assertProto(t, wrapper.GetPing(), ping)
}

func TestSrvPingPong(t *testing.T) {
	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send a ping from node A to B
	t.Run("A->B", func(t *testing.T) { assert.NoError(t, srvA.ping(peerB)) })
	time.Sleep(graceTime)

	// send a ping from node B to A
	t.Run("B->A", func(t *testing.T) { assert.NoError(t, srvB.ping(peerA)) })
}

func TestSrvPingTimeout(t *testing.T) {
	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	closeB() // close the connection right away to prevent any replies

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send a ping from node A to B
	err := srvA.ping(peerB)
	assert.EqualError(t, err, errTimeout.Error())
}

func TestSrvPeersRequest(t *testing.T) {
	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peers from node A
	t.Run("A->B", func(t *testing.T) {
		if ps, err := srvA.requestPeers(peerB); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerA}, ps)
		}
	})
	// request peers from node B
	t.Run("B->A", func(t *testing.T) {
		if ps, err := srvB.requestPeers(peerA); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerB}, ps)
		}
	})
}

func TestSrvVerifyBoot(t *testing.T) {
	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())

	srvB, closeB := newTestServer(t, "B", p2p.B, logger, peerA)
	defer closeB()

	time.Sleep(graceTime)
	if assert.EqualValues(t, 1, len(srvB.mgr.known)) {
		assert.EqualValues(t, peerA, &srvB.mgr.known[0].Peer)
		assert.EqualValues(t, 1, srvB.mgr.known[0].verifiedCount)
	}
}

func BenchmarkPingPong(b *testing.B) {
	p2p := transport.P2P()
	log := zap.NewNop().Sugar() // disable logging

	srvA, closeA := newTestServer(b, "A", p2p.A, log)
	defer closeA()
	srvB, closeB := newTestServer(b, "B", p2p.B, log)
	defer closeB()

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		// send a ping from node A to B
		_ = srvA.ping(peerB)
	}

	b.StopTimer()
}

func BenchmarkPeersRequest(b *testing.B) {
	p2p := transport.P2P()
	log := zap.NewNop().Sugar() // disable logging

	srvA, closeA := newTestServer(b, "A", p2p.A, log)
	defer closeA()
	srvB, closeB := newTestServer(b, "B", p2p.B, log)
	defer closeB()

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// send initial request to ensure that every peer is verified
	_, err := srvA.requestPeers(peerB)
	require.NoError(b, err)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = srvA.requestPeers(peerB)
	}

	b.StopTimer()
}
