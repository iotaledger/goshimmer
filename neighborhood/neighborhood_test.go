package neighborhood

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/distance"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
)

func TestRemove(t *testing.T) {
	d := make([]peer.PeerDistance, 10)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		input    *Neighborhood
		toRemove *peer.Peer
		expected []peer.PeerDistance
	}

	tests := []testCase{
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0]},
				Size:      4},
			toRemove: d[0].Remote,
			expected: []peer.PeerDistance{},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				Size:      4},
			toRemove: d[1].Remote,
			expected: []peer.PeerDistance{d[0], d[3]},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				Size:      4},
			toRemove: d[2].Remote,
			expected: []peer.PeerDistance{d[0], d[1], d[4]},
		},
	}

	for _, test := range tests {
		test.input.RemovePeer(test.toRemove.ID())
		assert.Equal(t, test.expected, test.input.Neighbors, "Remove")
	}
}

func TestAdd(t *testing.T) {
	d := make([]peer.PeerDistance, 10)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		input    *Neighborhood
		toAdd    peer.PeerDistance
		expected []peer.PeerDistance
	}

	tests := []testCase{
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0]},
				Size:      4},
			toAdd:    d[2],
			expected: []peer.PeerDistance{d[0], d[2]},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				Size:      4},
			toAdd:    d[2],
			expected: []peer.PeerDistance{d[0], d[1], d[3], d[2]},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				Size:      4},
			toAdd:    d[3],
			expected: []peer.PeerDistance{d[0], d[1], d[3], d[2]},
		},
	}

	for _, test := range tests {
		test.input.Add(test.toAdd)
		assert.Equal(t, test.expected, test.input.Neighbors, "Add")
	}
}

func TestUpdateDistance(t *testing.T) {
	d := make([]peer.PeerDistance, 5)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	s, _ := salt.NewSalt(100 * time.Second)

	d2 := make([]peer.PeerDistance, 4)
	for i := range d2 {
		d2[i].Remote = d[i+1].Remote
		d2[i].Distance = distance.BySalt(d[0].Remote.ID().Bytes(), d2[i].Remote.ID().Bytes(), s.GetBytes())
	}

	neighborhood := Neighborhood{
		Neighbors: d[1:],
		Size:      4}

	neighborhood.UpdateDistance(d[0].Remote.ID().Bytes(), s.GetBytes())

	assert.Equal(t, d2, neighborhood.Neighbors, "UpdateDistance")
}

func TestGetPeers(t *testing.T) {
	d := make([]peer.PeerDistance, 4)
	peers := make([]*peer.Peer, 4)

	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
		peers[i] = d[i].Remote
	}

	type testCase struct {
		input    *Neighborhood
		expected []*peer.Peer
	}

	tests := []testCase{
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{},
				Size:      4},
			expected: []*peer.Peer{},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0]},
				Size:      4},
			expected: []*peer.Peer{peers[0]},
		},
		{
			input: &Neighborhood{
				Neighbors: []peer.PeerDistance{d[0], d[1], d[3], d[2]},
				Size:      4},
			expected: []*peer.Peer{peers[0], peers[1], peers[3], peers[2]},
		},
	}

	for _, test := range tests {
		got := test.input.GetPeers()
		assert.Equal(t, test.expected, got, "GetPeers")
	}
}
