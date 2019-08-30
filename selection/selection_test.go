package selection

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
		neigbours    []peer.PeerDistance
		expCandidate *peer.Peer
	}

	tests := []testCase{
		{
			neigbours:    []peer.PeerDistance{d[0]},
			expCandidate: d[1].Remote,
		},
		{
			neigbours:    []peer.PeerDistance{d[0], d[1], d[3]},
			expCandidate: d[2].Remote,
		},
		{
			neigbours:    []peer.PeerDistance{d[0], d[1], d[2], d[4]},
			expCandidate: d[3].Remote,
		},
		{
			neigbours:    []peer.PeerDistance{d[0], d[1], d[2], d[3]},
			expCandidate: nil,
		},
	}

	for _, test := range tests {
		filter := make(Filter)
		for _, peer := range test.neigbours {
			filter[peer.Remote] = true
		}
		fList := filter.Complement(d)

		got := GetNextCandidate(test.neigbours, fList)

		assert.Equal(t, test.expCandidate, got, "Next Candidate", test)
	}

}
