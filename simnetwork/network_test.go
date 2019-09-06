package simnetwork

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNetwork(t *testing.T) {
	peerA := NewTransport("A")
	peerB := NewTransport("B")
	peers := []*Transport{peerA, peerB}

	NewNetwork(peers)

	assert.Equal(t, Network["A"], peerA)
	assert.Equal(t, Network["B"], peerB)
}
