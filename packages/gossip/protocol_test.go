package gossip

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
)

const (
	testNetwork = "tcp"
	testAddress = "127.0.0.1:8000"
)

func newTestServiceRecord() *service.Record {
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testAddress)
	return services
}

func TestVerifyRequest(t *testing.T) {
	pubA, privA, _ := ed25519.GenerateKey(nil)
	pubB, privB, _ := ed25519.GenerateKey(nil)
	peerA := &Local{
		id:  peer.PublicKey(pubA).ID(),
		key: peer.PrivateKey(privA),
	}
	peerB := peer.NewPeer(peer.PublicKey(pubB), newTestServiceRecord())
	p := &Protocol{
		version: 1,
		local:   peerA,
		neighbors: map[string]*peer.Peer{
			peerB.ID().String(): peerB,
		},
	}
	msg := &pb.ConnectionRequest{
		Version:   1,
		From:      peerB.ID().String(),
		To:        peerA.id.String(),
		Timestamp: 1,
	}
	signature, _ := signRequest(peer.PrivateKey(privB), msg)
	err := verifyRequest(p, msg, signature)
	if err != nil {
		fmt.Println("Error:", err)
	}
}
