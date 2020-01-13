package selection

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/distance"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/stretchr/testify/assert"
)

func TestGetFurthest(t *testing.T) {
	d := make([]peer.PeerDistance, 5)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		input    *Neighborhood
		expected peer.PeerDistance
		index    int
	}

	tests := []testCase{
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			expected: peer.PeerDistance{
				Remote:   nil,
				Distance: distance.Max},
			index: 1,
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[2], d[3]},
				size:      4},
			expected: d[3],
			index:    3,
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				size:      4},
			expected: d[4],
			index:    2,
		},
	}

	for _, test := range tests {
		p, l := test.input.getFurthest()
		assert.Equal(t, test.expected, p, "Get furthest neighbor")
		assert.Equal(t, test.index, l, "Get furthest neighbor")
	}
}

func TestGetPeerIndex(t *testing.T) {
	d := make([]peer.PeerDistance, 5)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		input  *Neighborhood
		target *peer.Peer
		index  int
	}

	tests := []testCase{
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			target: d[0].Remote,
			index:  0,
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[2], d[3]},
				size:      4},
			target: d[3].Remote,
			index:  3,
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				size:      4},
			target: d[3].Remote,
			index:  -1,
		},
	}

	for _, test := range tests {
		i := test.input.getPeerIndex(test.target.ID())
		assert.Equal(t, test.index, i, "Get Peer Index")
	}
}

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
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			toRemove: d[0].Remote,
			expected: []peer.PeerDistance{},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				size:      4},
			toRemove: d[1].Remote,
			expected: []peer.PeerDistance{d[0], d[3]},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				size:      4},
			toRemove: d[2].Remote,
			expected: []peer.PeerDistance{d[0], d[1], d[4]},
		},
	}

	for _, test := range tests {
		test.input.RemovePeer(test.toRemove.ID())
		assert.Equal(t, test.expected, test.input.neighbors, "Remove")
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
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			toAdd:    d[2],
			expected: []peer.PeerDistance{d[0], d[2]},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				size:      4},
			toAdd:    d[2],
			expected: []peer.PeerDistance{d[0], d[1], d[3], d[2]},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				size:      4},
			toAdd:    d[3],
			expected: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
		},
	}

	for _, test := range tests {
		test.input.Add(test.toAdd)
		assert.Equal(t, test.expected, test.input.neighbors, "Add")
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
		neighbors: d[1:],
		size:      4}

	neighborhood.UpdateDistance(d[0].Remote.ID().Bytes(), s.GetBytes())

	assert.Equal(t, d2, neighborhood.neighbors, "UpdateDistance")
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
				neighbors: []peer.PeerDistance{},
				size:      4},
			expected: []*peer.Peer{},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			expected: []*peer.Peer{peers[0]},
		},
		{
			input: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[3], d[2]},
				size:      4},
			expected: []*peer.Peer{peers[0], peers[1], peers[3], peers[2]},
		},
	}

	for _, test := range tests {
		got := test.input.GetPeers()
		assert.Equal(t, test.expected, got, "GetPeers")
	}
}
