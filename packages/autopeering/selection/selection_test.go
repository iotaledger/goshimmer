package selection

import (
	"crypto/ed25519"
	"testing"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/stretchr/testify/assert"
)

const (
	testNetwork = "udp"
	testAddress = "127.0.0.1:8000"
)

func newTestServiceRecord() *service.Record {
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testAddress)

	return services
}

func newTestPeer() *peer.Peer {
	key, _, _ := ed25519.GenerateKey(nil)
	return peer.NewPeer(peer.PublicKey(key), newTestServiceRecord())
}

func TestFilterAddPeers(t *testing.T) {
	p := make([]*peer.Peer, 5)
	for i := range p {
		p[i] = newTestPeer()
	}

	type testCase struct {
		input    []*peer.Peer
		notExist []*peer.Peer
	}

	tests := []testCase{
		{
			input:    []*peer.Peer{p[0]},
			notExist: []*peer.Peer{p[1]},
		},
		{
			input:    []*peer.Peer{p[0], p[1], p[2]},
			notExist: []*peer.Peer{p[3], p[4]},
		},
	}

	for _, test := range tests {
		f := NewFilter()
		f.AddPeers(test.input)
		for _, e := range test.input {
			assert.Equal(t, true, f.internal[e.ID()], "Filter add peers")
		}

		for _, e := range test.notExist {
			assert.Equal(t, false, f.internal[e.ID()], "Filter add peers")
		}
	}
}

func TestFilterRemovePeers(t *testing.T) {
	p := make([]*peer.Peer, 5)
	for i := range p {
		p[i] = newTestPeer()
	}

	type testCase struct {
		input    []*peer.Peer
		toRemove *peer.Peer
		left     []*peer.Peer
	}

	tests := []testCase{
		{
			input:    []*peer.Peer{p[0]},
			toRemove: p[0],
			left:     []*peer.Peer{},
		},
		{
			input:    []*peer.Peer{p[0], p[1], p[2]},
			toRemove: p[1],
			left:     []*peer.Peer{p[0], p[2]},
		},
	}

	for _, test := range tests {
		f := NewFilter()
		f.AddPeers(test.input)
		f.RemovePeer(test.toRemove.ID())
		for _, e := range test.left {
			assert.Equal(t, true, f.internal[e.ID()], "Filter remove peers")
		}
		assert.Equal(t, false, f.internal[test.toRemove.ID()], "Filter remove peers")
	}
}

func TestFilterApply(t *testing.T) {
	d := make([]peer.PeerDistance, 5)
	for i := range d {
		d[i].Remote = newTestPeer()
		d[i].Distance = uint32(i + 1)
	}

	type testCase struct {
		input    []*peer.Peer
		apply    []peer.PeerDistance
		expected []peer.PeerDistance
	}

	tests := []testCase{
		{
			input:    []*peer.Peer{d[0].Remote},
			apply:    []peer.PeerDistance{d[0], d[1], d[2]},
			expected: []peer.PeerDistance{d[1], d[2]},
		},
		{
			input:    []*peer.Peer{d[0].Remote, d[1].Remote},
			apply:    []peer.PeerDistance{d[2], d[3], d[4]},
			expected: []peer.PeerDistance{d[2], d[3], d[4]},
		},
	}

	for _, test := range tests {
		f := NewFilter()
		f.AddPeers(test.input)
		filteredList := f.Apply(test.apply)
		assert.Equal(t, test.expected, filteredList, "Filter apply")
	}
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
				neighbors: []peer.PeerDistance{d[0]},
				size:      4},
			expCandidate: d[1].Remote,
		},
		{
			nh: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[3]},
				size:      4},
			expCandidate: d[2].Remote,
		},
		{
			nh: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[4], d[2]},
				size:      4},
			expCandidate: d[3].Remote,
		},
		{
			nh: &Neighborhood{
				neighbors: []peer.PeerDistance{d[0], d[1], d[2], d[3]},
				size:      4},
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
