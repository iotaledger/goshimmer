package peering

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
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
	expected := &pb.MessageWrapper{Message: &pb.MessageWrapper_Ping{Ping: ping}}

	packet, err := Encode(id, expected)
	if err != nil {
		t.Error(err)
	}

	ingress := &IngressPacket{}
	if err := Decode(packet, ingress); err != nil {
		t.Error(err)
	}

	assertProto(t, ingress.Message.GetPing(), expected.GetPing())
}
