package neighborhood

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"github.com/wollac/autopeering/server"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

const graceTime = 20 * time.Millisecond

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}

func getKnowPeers() []*peer.Peer {
	return []*peer.Peer{}
}

// newTest creates a new neighborhood server and also returns the teardown.
func newTest(t require.TestingT, name string, trans transport.Transport, logger *zap.SugaredLogger) (*server.Server, *Protocol, func()) {
	priv, err := peer.GeneratePrivateKey()
	require.NoError(t, err)

	log := logger.Named(name)
	db := peer.NewMapDB(log.Named("db"))
	local := peer.NewLocal(priv, db)
	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)

	cfg := Config{
		Log:       log,
		PeersFunc: getKnowPeers,
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

	// request peering to peer A
	t.Run("A->B", func(t *testing.T) {
		if accept, err := protA.RequestPeering(peerB, saltA); assert.NoError(t, err) {
			assert.True(t, accept)
		}
	})
	// request peering to peer B
	t.Run("B->A", func(t *testing.T) {
		if accept, err := protB.RequestPeering(peerA, saltB); assert.NoError(t, err) {
			assert.True(t, accept)
		}
	})
}

func TestProtExpiredSalt(t *testing.T) {
	p2p := transport.P2P()

	_, protA, closeA := newTest(t, "A", p2p.A, logger)
	defer closeA()
	srvB, _, closeB := newTest(t, "B", p2p.B, logger)
	defer closeB()

	saltA, _ := salt.NewSalt(-1 * time.Second)
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peering to peer A
	_, err := protA.RequestPeering(peerB, saltA)
	assert.EqualError(t, err, server.ErrTimeout.Error())
}

func TestProtDropPeer(t *testing.T) {
	p2p := transport.P2P()

	_, protA, closeA := newTest(t, "A", p2p.A, logger)
	defer closeA()
	srvB, protB, closeB := newTest(t, "B", p2p.B, logger)
	defer closeB()

	saltA, _ := salt.NewSalt(100 * time.Second)
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	// request peering to peer A
	accept, err := protA.RequestPeering(peerB, saltA)
	require.NoError(t, err)
	assert.True(t, accept)

	require.NotEmpty(t, protB.GetNeighbors())

	// drop peer A
	protA.DropPeer(peerB)
	time.Sleep(graceTime)
	require.Empty(t, protB.GetNeighbors())
}
