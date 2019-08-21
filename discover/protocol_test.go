package discover

import (
	"testing"

	"github.com/golang/protobuf/proto"
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
	packet, err := encode(id, ping)
	if err != nil {
		t.Fatal(err)
	}

	wrapper, _, err := decode(packet)
	if err != nil {
		t.Fatal(err)
	}

	assertProto(t, wrapper.GetPing(), ping)
}

func TestPingPong(t *testing.T) {
	p2p := transport.P2P()
	defer p2p.Close()

	a, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer a.Close()
	b, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	defer b.Close()

	if err := a.ping(p2p.B.LocalAddr(), b.LocalID().StringID); err != nil {
		t.Fatal(err)
	}
}
