package response

import (
	"net"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
)

func TestPeer_MarshalUnmarshal(t *testing.T) {
	issuer := &peer.Peer{}
	issuer.SetAddress(net.IPv4(127, 0, 0, 1))
	issuer.SetIdentity(identity.GenerateRandomIdentity())
	issuer.SetGossipPort(123)
	issuer.SetPeeringPort(456)
	issuer.SetSalt(salt.New(30 * time.Second))

	peers := make([]*peer.Peer, 3)

	peers[0] = &peer.Peer{}
	peers[0].SetAddress(net.IPv4(127, 0, 0, 1))
	peers[0].SetIdentity(identity.GenerateRandomIdentity())
	peers[0].SetGossipPort(124)
	peers[0].SetPeeringPort(457)
	peers[0].SetSalt(salt.New(30 * time.Second))

	peers[1] = &peer.Peer{}
	peers[1].SetAddress(net.IPv4(127, 0, 0, 1))
	peers[1].SetIdentity(identity.GenerateRandomIdentity())
	peers[1].SetGossipPort(125)
	peers[1].SetPeeringPort(458)
	peers[1].SetSalt(salt.New(30 * time.Second))

	peers[2] = &peer.Peer{}
	peers[2].SetAddress(net.IPv4(127, 0, 0, 1))
	peers[2].SetIdentity(identity.GenerateRandomIdentity())
	peers[2].SetGossipPort(126)
	peers[2].SetPeeringPort(459)
	peers[2].SetSalt(salt.New(30 * time.Second))

	response := &Response{
		Issuer: issuer,
		Type:   TYPE_ACCEPT,
		Peers:  peers,
	}
	response.Sign()

	_, err := Unmarshal(response.Marshal())
	if err != nil {
		t.Error(err)
	}
}
