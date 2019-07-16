package peer

import (
	"net"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"

	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/magiconair/properties/assert"
)

func TestPeer_MarshalUnmarshal(t *testing.T) {
	peer := &Peer{
		Address:     net.IPv4(127, 0, 0, 1),
		Identity:    identity.GenerateRandomIdentity(),
		GossipPort:  123,
		PeeringPort: 456,
		Salt:        salt.New(30 * time.Second),
	}

	restoredPeer, _ := Unmarshal(peer.Marshal())

	assert.Equal(t, peer.Address, restoredPeer.Address)
	assert.Equal(t, peer.Identity.StringIdentifier, restoredPeer.Identity.StringIdentifier)
	assert.Equal(t, peer.Identity.PublicKey, restoredPeer.Identity.PublicKey)
	assert.Equal(t, peer.GossipPort, restoredPeer.GossipPort)
	assert.Equal(t, peer.PeeringPort, restoredPeer.PeeringPort)
	assert.Equal(t, peer.Salt, restoredPeer.Salt)
}
