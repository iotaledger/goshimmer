package simnetwork

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestNewNetwork(t *testing.T) {
	peerA := NewTransport("A")
	peerB := NewTransport("B")
	peers := []*Transport{peerA, peerB}

	NewNetwork(peers)

	assert.Equal(t, Network["A"], peerA)
	assert.Equal(t, Network["B"], peerB)
}
