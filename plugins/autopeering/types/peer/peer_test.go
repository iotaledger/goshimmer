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
		address:     net.IPv4(127, 0, 0, 1),
		identity:    identity.GenerateRandomIdentity(),
		gossipPort:  123,
		peeringPort: 456,
		salt:        salt.New(30 * time.Second),
	}

	restoredPeer, err := Unmarshal(peer.Marshal())
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, peer.GetAddress(), restoredPeer.GetAddress())
	assert.Equal(t, peer.GetIdentity().StringIdentifier, restoredPeer.GetIdentity().StringIdentifier)
	assert.Equal(t, peer.GetIdentity().PublicKey, restoredPeer.GetIdentity().PublicKey)
	assert.Equal(t, peer.GetGossipPort(), restoredPeer.GetGossipPort())
	assert.Equal(t, peer.GetPeeringPort(), restoredPeer.GetPeeringPort())
	assert.Equal(t, peer.GetSalt().GetBytes(), restoredPeer.GetSalt().GetBytes())
	// time.time cannot be compared with reflect.DeepEqual, so we cannot use assert.Equal here
	if !peer.GetSalt().GetExpirationTime().Equal(restoredPeer.GetSalt().GetExpirationTime()) {
		t.Errorf("got %v want %v", restoredPeer.GetSalt().GetExpirationTime(), peer.GetSalt().GetExpirationTime())
	}
}
