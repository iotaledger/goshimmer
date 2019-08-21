package discover

import (
	"net"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
	"github.com/wollac/autopeering/transport"
)

var (
	testIP   = net.ParseIP("127.0.0.1")
	testPing = &pb.Ping{
		Version: 0,
		From:    &pb.RpcEndpoint{Ip: testIP.String(), Port: 1},
		To:      &pb.RpcEndpoint{Ip: testIP.String(), Port: 2},
	}
)

func assertProto(t *testing.T, got, want proto.Message) {
	if !proto.Equal(got, want) {
		t.Errorf("got %v want %v\n", got, want)
	}
}

func getTestPing() *pb.Ping {
	return &pb.Ping{
		Version: 0,
		From: &pb.RpcEndpoint{
			Ip:   "127.0.0.1",
			Port: 8888,
		},
		To: &pb.RpcEndpoint{
			Ip:   "127.0.0.1",
			Port: 8889,
		},
	}
}

func TestEncodeDecodePing(t *testing.T) {
	id := identity.GeneratePrivateIdentity()

	ping := getTestPing()
	packet, _, err := encode(id, ping)
	if err != nil {
		t.Error(err)
	}

	wrapper, _, err := decode(packet)
	if err != nil {
		t.Error(err)
	}

	assertProto(t, wrapper.GetPing(), ping)
}

func TestPingPong(t *testing.T) {
	p2p := transport.NewP2P(testIP)
	defer p2p.Close()

	a, _ := Listen(p2p.A, identity.GeneratePrivateIdentity())
	defer a.Close()
	b, _ := Listen(p2p.B, identity.GeneratePrivateIdentity())
	defer b.Close()

	if err := a.ping(p2p.B.LocalEndpoint(), b.OwnId().StringId); err != nil {
		t.Error(err)
	}
}
