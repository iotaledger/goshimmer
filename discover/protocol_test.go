package discover

import (
	"log"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wollac/autopeering/id"
	pb "github.com/wollac/autopeering/proto"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

var (
	testAddr = "127.0.0.1:8888"
	testPing = &pb.Ping{Version: 0, From: testAddr, To: testAddr}
)

var logger *zap.Logger

func assertProto(t *testing.T, got, want proto.Message) {
	if !proto.Equal(got, want) {
		t.Errorf("got %v want %v\n", got, want)
	}
}

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l
}

func newID() *id.Private {
	return id.GeneratePrivate()
}

func TestEncodeDecodePing(t *testing.T) {
	priv := newID()

	ping := testPing
	packet := encode(priv, ping)

	wrapper, id, err := decode(packet)
	require.NoError(t, err)

	assert.Equal(t, id.PublicKey, priv.PublicKey)
	assertProto(t, wrapper.GetPing(), ping)
}

func TestPingPong(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, Config{newID(), logger})
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, Config{newID(), logger})
	defer nodeB.Close()

	peerA := newPeer(&nodeA.LocalID().Identity, nodeA.LocalAddr())
	peerB := newPeer(&nodeB.LocalID().Identity, nodeB.LocalAddr())

	// send a ping from node A to B
	assert.NoError(t, nodeA.ping(peerB))
	// send a ping from node B to A
	assert.NoError(t, nodeB.ping(peerA))
}

func TestPingTimeout(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, Config{newID(), logger})
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, Config{newID(), logger})
	nodeB.Close() // close the connection right away to prevent any replies

	peerB := newPeer(&nodeB.LocalID().Identity, nodeB.LocalAddr())

	// send a ping from node A to B
	err := nodeA.ping(peerB)
	assert.EqualError(t, err, errTimeout.Error())
}

func BenchmarkPingPong(b *testing.B) {
	p2p := transport.P2P()
	logger, _ := zap.NewProduction() // use production level logging

	nodeA, _ := Listen(p2p.A, Config{newID(), logger})
	nodeB, _ := Listen(p2p.B, Config{newID(), logger})

	peerB := newPeer(&nodeB.LocalID().Identity, nodeB.LocalAddr())

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		// send a ping from node A to B
		_ = nodeA.ping(peerB)
	}

	b.StopTimer()

	nodeA.Close()
	nodeB.Close()
}
