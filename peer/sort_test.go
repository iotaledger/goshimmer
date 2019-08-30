package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		remotePeers[i] = newTestPeer()
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
