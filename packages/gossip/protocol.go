package gossip

import (
	"crypto/ed25519"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
)

type Local struct {
	id  peer.ID
	key peer.PrivateKey
}

type Protocol struct {
	version   uint32
	local     *Local
	mgr       *Manager
	neighbors map[string]*peer.Peer
}

func sendRequest(p *Protocol, to *peer.Peer) error {
	msg := &pb.ConnectionRequest{
		Version:   p.version,
		From:      p.local.id.String(),
		To:        to.ID().String(),
		Timestamp: 1,
	}

	return nil
}

func verifyRequest(p *Protocol, msg *pb.ConnectionRequest, signature []byte) error {
	requester, ok := p.neighbors[msg.GetFrom()]
	if !ok {
		return errSender
	}
	if msg.GetVersion() != p.version {
		return errVersion
	}
	if msg.GetTo() != p.local.id.String() {
		return errRecipient
	}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(requester.PublicKey()), msgBytes, signature) {
		return errSignature
	}
	return nil
}

func signRequest(key peer.PrivateKey, msg *pb.ConnectionRequest) (signature []byte, err error) {
	msgByte, err := proto.Marshal(msg)
	if err != nil {
		return signature, err
	}
	return ed25519.Sign(ed25519.PrivateKey(key), msgByte), nil
}
