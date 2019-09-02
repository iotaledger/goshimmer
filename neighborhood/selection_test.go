package neighborhood

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/peer"
	"golang.org/x/crypto/ed25519"
)

const (
	testAddress = "127.0.0.1:8000"
)

func newTestPeer() *peer.Peer {
	key := make([]byte, ed25519.PublicKeySize)
	return peer.NewPeer(key, testAddress)
}

func TestSelection(t *testing.T) {
	d := make([]peer.PeerDistance, 10)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		nh           Neighborhood
		expCandidate *peer.Peer
	}

	tests := []testCase{
		{
			nh:           Neighborhood{[]peer.PeerDistance{d[0]}, 4},
			expCandidate: d[1].Remote,
		},
		{
			nh:           Neighborhood{[]peer.PeerDistance{d[0], d[1], d[3]}, 4},
			expCandidate: d[2].Remote,
		},
		{
			nh:           Neighborhood{[]peer.PeerDistance{d[0], d[1], d[4], d[2]}, 4},
			expCandidate: d[3].Remote,
		},
		{
			nh:           Neighborhood{[]peer.PeerDistance{d[0], d[1], d[2], d[3]}, 4},
			expCandidate: nil,
		},
	}

	for _, test := range tests {
		filter := make(Filter)
		for _, peer := range test.nh.Neighbors {
			filter[peer.Remote] = true
		}
		fList := filter.Apply(d)

		got := test.nh.Select(fList)

		assert.Equal(t, test.expCandidate, got.Remote, "Next Candidate", test)
	}

}
