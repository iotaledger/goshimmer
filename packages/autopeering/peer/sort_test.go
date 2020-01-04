package peer

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestPeerWithID(ID byte) *Peer {
	key := make([]byte, ed25519.PublicKeySize)
	key[0] = ID
	return NewPeer(key, newTestServiceRecord())
}

func TestOrderedDistanceList(t *testing.T) {
	type testCase struct {
		anchor  []byte
		salt    []byte
		ordered bool
	}

	tests := []testCase{
		{
			anchor:  []byte("X"),
			salt:    []byte("salt"),
			ordered: true,
		},
	}

	remotePeers := make([]*Peer, 10)
	for i := range remotePeers {
		remotePeers[i] = newTestPeerWithID(byte(i + 1))
	}

	for _, test := range tests {
		d := SortBySalt(test.anchor, test.salt, remotePeers)
		prev := d[0]
		for _, next := range d[1:] {
			got := prev.Distance < next.Distance
			assert.Equal(t, test.ordered, got, "Ordered distance list")
			prev = next
		}
	}
}
