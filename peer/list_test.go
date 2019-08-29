package peer

import (
	"sort"
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

	remotePeers := make(List, 10)
	for i := range remotePeers {
		remotePeers[i] = newTestPeer()
	}

	for _, test := range tests {
		d := DistanceList(test.anchor, test.salt, remotePeers)
		sort.Sort(ByDistance(d))

		prev := d[0]
		for _, next := range d[1:] {
			got := prev.Distance < next.Distance
			assert.Equal(t, test.ordered, got, "Ordered distance list")
			prev = next
		}
	}
}
