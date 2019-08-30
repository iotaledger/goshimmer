package neighborhood

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/peer"
)

func newTestPeer() *peer.Peer {
	prv := id.GeneratePrivate()
	p := &peer.Peer{}
	p.Identity, _ = id.NewIdentity(prv.PublicKey)
	p.Address = "127.0.0.1:8000"
	return p
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
		for _, peer := range test.nh.Neighbours {
			filter[peer.Remote] = true
		}
		fList := filter.Complement(d)

		got := test.nh.Select(fList)

		assert.Equal(t, test.expCandidate, got, "Next Candidate", test)
	}

}
