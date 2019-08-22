package discover

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
	"github.com/wollac/autopeering/transport"
)

var (
	testAddr = "127.0.0.1:8888"
	testPing = &pb.Ping{Version: 0, From: testAddr, To: testAddr}
)

func assertProto(t *testing.T, got, want proto.Message) {
	if !proto.Equal(got, want) {
		t.Errorf("got %v want %v\n", got, want)
	}
}

func TestEncodeDecodePing(t *testing.T) {
	priv := identity.GeneratePrivateIdentity()

	ping := testPing
	packet := encode(priv, ping)

	wrapper, id, err := decode(packet)
	require.NoError(t, err)

	assert.Equal(t, id.PublicKey, priv.PublicKey)
	assertProto(t, wrapper.GetPing(), ping)
}

func TestPingPong(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	defer nodeB.Close()

	// send a ping from node A to B
	assert.NoError(t, nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID))
	// send a ping from node B to A
	assert.NoError(t, nodeB.ping(p2p.A.LocalAddr(), nodeA.LocalID().StringID))
}

func TestPingTimeout(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	nodeB.Close() // close the connection right away to prevent any replies

	// send a ping from node A to B
	err := nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID)
	assert.EqualError(t, err, errTimeout.Error())
}

func BenchmarkPingPong(b *testing.B) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		// send a ping from node A to B
		_ = nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID)
	}

	b.StopTimer()

	nodeA.Close()
	nodeB.Close()
}
