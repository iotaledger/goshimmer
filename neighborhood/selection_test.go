package neighborhood

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/peer"
)

const (
	testAddress = "127.0.0.1:8000"
)

func newTestPeer() *peer.Peer {
	key, _, _ := ed25519.GenerateKey(nil)
	return peer.NewPeer(peer.PublicKey(key), testAddress)
}

func TestSelection(t *testing.T) {
	d := make([]peer.PeerDistance, 10)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		nh           *Neighborhood
		expCandidate *peer.Peer
	}

	tests := []testCase{
		{
			nh: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0]},
				Size:      4},
			expCandidate: d[1].Remote,
		},
		{
			nh: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				Size:      4},
			expCandidate: d[2].Remote,
		},
		{
			nh: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				Size:      4},
			expCandidate: d[3].Remote,
		},
		{
			nh: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[2], d[3]},
				Size:      4},
			expCandidate: nil,
		},
	}

	for _, test := range tests {
		filter := NewFilter()
		filter.AddPeers(test.nh.GetPeers())
		fList := filter.Apply(d)

		got := test.nh.Select(fList)

		assert.Equal(t, test.expCandidate, got.Remote, "Next Candidate", test)
	}
}
