package discover

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/magiconair/properties/assert"
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
	id := identity.GeneratePrivateIdentity()

	ping := testPing
	packet := encode(id, ping)

	wrapper, _, err := decode(packet)
	if err != nil {
		t.Fatal(err)
	}

	assertProto(t, wrapper.GetPing(), ping)
}

func TestPingPong(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	defer nodeB.Close()

	if err := nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID); err != nil {
		t.Fatal(err)
	}
	if err := nodeB.ping(p2p.A.LocalAddr(), nodeA.LocalID().StringID); err != nil {
		t.Fatal(err)
	}
}

func TestPingTimeout(t *testing.T) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer nodeA.Close()
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	nodeB.Close() // close the connection right away to prevent any replies

	err := nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID)
	assert.Equal(t, err, errTimeout)
}

func BenchmarkPingPong(b *testing.B) {
	p2p := transport.P2P()

	nodeA, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	nodeB, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = nodeA.ping(p2p.B.LocalAddr(), nodeB.LocalID().StringID)
	}

	b.StopTimer()

	nodeA.Close()
	nodeB.Close()
}
