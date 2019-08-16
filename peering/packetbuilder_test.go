package peer

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
	p := NewPacketBuilder(identity.GeneratePrivateIdentity())

	ping := getTestPing()
	expected := &pb.MessageWrapper{Message: &pb.MessageWrapper_Ping{Ping: ping}}

	packet, err := p.Encode(expected)
	if err != nil {
		t.Error(err)
	}

	actual := &pb.MessageWrapper{}
	if _, err := Decode(packet, actual); err != nil {
		t.Error(err)
	}

	assertProto(t, actual.GetPing(), expected.GetPing())
}
