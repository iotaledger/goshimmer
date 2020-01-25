package discover

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/peertest"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	testNetwork = "test"
	graceTime   = 100 * time.Millisecond
)

var (
	log    *logger.Logger
	peerDB *peer.DB
)

func init() {
	log = logger.NewExampleLogger("discover")
	db, err := peer.NewMemoryDB()
	if err != nil {
		panic(err)
	}
	peerDB = db

	// decrease parameters to simplify and speed up tests
	SetParameter(Parameters{
		ReverifyInterval: 500 * time.Millisecond,
		QueryInterval:    1000 * time.Millisecond,
		MaxManaged:       10,
		MaxReplacements:  2,
	})
}

func TestProtVerifyMaster(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()

	peerA := getPeer(protA)

	// use peerA as masters peer
	protB, closeB := newTestProtocol(p2p.B, log, peerA)

	time.Sleep(graceTime) // wait for the packages to ripple through the network
	closeB()              // close srvB to avoid race conditions, when asserting

	if assert.EqualValues(t, 1, len(protB.mgr.active)) {
		assert.EqualValues(t, peerA, &protB.mgr.active[0].Peer)
		assert.EqualValues(t, 1, protB.mgr.active[0].verifiedCount)
	}
}

func TestProtPingPong(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerA := getPeer(protA)
	peerB := getPeer(protB)

	// send a Ping from node A to B
	t.Run("A->B", func(t *testing.T) { assert.NoError(t, protA.Ping(peerB)) })
	time.Sleep(graceTime)

	// send a Ping from node B to A
	t.Run("B->A", func(t *testing.T) { assert.NoError(t, protB.Ping(peerA)) })
	time.Sleep(graceTime)
}

func TestProtPingTimeout(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	closeB() // close the connection right away to prevent any replies

	// send a Ping from node A to B
	err := protA.Ping(getPeer(protB))
	assert.EqualError(t, err, server.ErrTimeout.Error())
}

func TestProtVerifiedPeers(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerB := getPeer(protB)

	// send a Ping from node A to B
	assert.NoError(t, protA.Ping(peerB))
	time.Sleep(graceTime)

	// protA should have peerB as the single verified peer
	assert.ElementsMatch(t, []*peer.Peer{peerB}, protA.GetVerifiedPeers())
	for _, p := range protA.GetVerifiedPeers() {
		assert.Equal(t, p, protA.GetVerifiedPeer(p.ID(), p.Address()))
	}
}

func TestProtVerifiedPeer(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerA := getPeer(protA)
	peerB := getPeer(protB)

	// send a Ping from node A to B
	assert.NoError(t, protA.Ping(peerB))
	time.Sleep(graceTime)

	// we should have peerB as a verified peer
	assert.Equal(t, peerB, protA.GetVerifiedPeer(peerB.ID(), peerB.Address()))
	// we should not have ourselves as a verified peer
	assert.Nil(t, protA.GetVerifiedPeer(peerA.ID(), peerA.Address()))
	// the address of peerB should match
	assert.Nil(t, protA.GetVerifiedPeer(peerB.ID(), ""))
}

func TestProtDiscoveryRequest(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerA := getPeer(protA)
	peerB := getPeer(protB)

	// request peers from node A
	t.Run("A->B", func(t *testing.T) {
		if ps, err := protA.DiscoveryRequest(peerB); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerA}, ps)
		}
	})
	// request peers from node B
	t.Run("B->A", func(t *testing.T) {
		if ps, err := protB.DiscoveryRequest(peerA); assert.NoError(t, err) {
			assert.ElementsMatch(t, []*peer.Peer{peerB}, ps)
		}
	})
}

func TestProtServices(t *testing.T) {
	clearPeerDB(t)
	p2p := transport.P2P()
	defer p2p.Close()

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()

	err := protA.local().UpdateService(service.FPCKey, "fpc", p2p.A.LocalAddr().String())
	require.NoError(t, err)

	peerA := getPeer(protA)

	// use peerA as masters peer
	protB, closeB := newTestProtocol(p2p.B, log, peerA)
	defer closeB()

	time.Sleep(graceTime) // wait for the packages to ripple through the network
	ps := protB.GetVerifiedPeers()

	if assert.ElementsMatch(t, []*peer.Peer{peerA}, ps) {
		assert.Equal(t, protA.local().Services(), ps[0].Services())
	}
}

func TestProtDiscovery(t *testing.T) {
	clearPeerDB(t)
	net := transport.NewNetwork("M", "A", "B", "C")
	defer net.Close()

	protM, closeM := newTestProtocol(net.GetTransport("M"), log)
	defer closeM()
	time.Sleep(graceTime) // wait for the master to initialize

	protA, closeA := newTestProtocol(net.GetTransport("A"), log, getPeer(protM))
	defer closeA()
	protB, closeB := newTestProtocol(net.GetTransport("B"), log, getPeer(protM))
	defer closeB()
	protC, closeC := newTestProtocol(net.GetTransport("C"), log, getPeer(protM))
	defer closeC()

	time.Sleep(queryInterval + graceTime)    // wait for the next discovery cycle
	time.Sleep(reverifyInterval + graceTime) // wait for the next verification cycle

	// now the full network should be discovered
	assert.ElementsMatch(t, []*peer.Peer{getPeer(protA), getPeer(protB), getPeer(protC)}, protM.GetVerifiedPeers())
	assert.ElementsMatch(t, []*peer.Peer{getPeer(protM), getPeer(protB), getPeer(protC)}, protA.GetVerifiedPeers())
	assert.ElementsMatch(t, []*peer.Peer{getPeer(protM), getPeer(protA), getPeer(protC)}, protB.GetVerifiedPeers())
	assert.ElementsMatch(t, []*peer.Peer{getPeer(protM), getPeer(protA), getPeer(protB)}, protC.GetVerifiedPeers())
}

func BenchmarkPingPong(b *testing.B) {
	clearPeerDB(b)
	p2p := transport.P2P()
	defer p2p.Close()
	log := zap.NewNop().Sugar() // disable logging

	// disable query/reverify
	reverifyInterval = time.Hour
	queryInterval = time.Hour

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerB := getPeer(protB)

	// send initial Ping to ensure that every peer is verified
	err := protA.Ping(peerB)
	require.NoError(b, err)
	time.Sleep(graceTime)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// send a Ping from node A to B
		_ = protA.Ping(peerB)
	}

	b.StopTimer()
}

func BenchmarkDiscoveryRequest(b *testing.B) {
	clearPeerDB(b)
	p2p := transport.P2P()
	defer p2p.Close()
	log := zap.NewNop().Sugar() // disable logging

	// disable query/reverify
	reverifyInterval = time.Hour
	queryInterval = time.Hour

	protA, closeA := newTestProtocol(p2p.A, log)
	defer closeA()
	protB, closeB := newTestProtocol(p2p.B, log)
	defer closeB()

	peerB := getPeer(protB)

	// send initial DiscoveryRequest to ensure that every peer is verified
	_, err := protA.DiscoveryRequest(peerB)
	require.NoError(b, err)
	time.Sleep(graceTime)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = protA.DiscoveryRequest(peerB)
	}

	b.StopTimer()
}

func clearPeerDB(t require.TestingT) {
	require.NoError(t, peerDB.Clear())
}

// newTestProtocol creates a new discovery server and also returns the teardown.
func newTestProtocol(trans transport.Transport, logger *logger.Logger, masters ...*peer.Peer) (*Protocol, func()) {
	local := peertest.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), peerDB)
	log := logger.Named(trans.LocalAddr().String())

	prot := New(local, Config{Log: log, MasterPeers: masters})

	srv := server.Serve(local, trans, log, prot)
	prot.Start(srv)

	teardown := func() {
		srv.Close()
		prot.Close()
	}
	return prot, teardown
}

func getPeer(p *Protocol) *peer.Peer {
	return &p.local().Peer
}
