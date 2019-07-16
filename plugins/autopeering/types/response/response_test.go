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
	issuer := &peer.Peer{
		Address:     net.IPv4(127, 0, 0, 1),
		Identity:    identity.GenerateRandomIdentity(),
		GossipPort:  123,
		PeeringPort: 456,
		Salt:        salt.New(30 * time.Second),
	}

	peers := make([]*peer.Peer, 0)
	peers = append(peers, &peer.Peer{
		Address:     net.IPv4(127, 0, 0, 1),
		Identity:    identity.GenerateRandomIdentity(),
		GossipPort:  124,
		PeeringPort: 457,
		Salt:        salt.New(30 * time.Second),
	}, &peer.Peer{
		Address:     net.IPv4(127, 0, 0, 1),
		Identity:    identity.GenerateRandomIdentity(),
		GossipPort:  125,
		PeeringPort: 458,
		Salt:        salt.New(30 * time.Second),
	}, &peer.Peer{
		Address:     net.IPv4(127, 0, 0, 1),
		Identity:    identity.GenerateRandomIdentity(),
		GossipPort:  126,
		PeeringPort: 459,
		Salt:        salt.New(30 * time.Second),
	})

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
