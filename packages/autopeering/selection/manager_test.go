package selection

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
)

var (
	allPeers []*peer.Peer
)

type testPeer struct {
	local *peer.Local
	peer  *peer.Peer
	db    peer.DB
	log   *logger.Logger
	rand  *rand.Rand // random number generator
}

func newPeer(name string) testPeer {
	log := log.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, _ := peer.NewLocal("", name, db)
	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)
	p := &local.Peer
	return testPeer{local, p, db, log, rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func removeDuplicatePeers(peers []*peer.Peer) []*peer.Peer {
	seen := make(map[peer.ID]bool, len(peers))
	result := make([]*peer.Peer, 0, len(peers))

	for _, p := range peers {
		if !seen[p.ID()] {
			seen[p.ID()] = true
			result = append(result, p)
		}
	}

	return result
}

type testNet struct {
	loc  *peer.Local
	self *peer.Peer
	mgr  map[peer.ID]*manager
	rand *rand.Rand
}

func (n testNet) local() *peer.Local {
	return n.loc
}

func (n testNet) DropPeer(p *peer.Peer) {
	n.mgr[p.ID()].dropNeighbor(n.local().ID())
}

func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	return n.mgr[p.ID()].acceptRequest(n.self, s), nil
}

func (n testNet) GetKnownPeers() []*peer.Peer {
	list := make([]*peer.Peer, len(allPeers)-1)
	i := 0
	for _, p := range allPeers {
		if p.ID() == n.self.ID() {
			continue
		}

		list[i] = p
		i++
	}
	return list
}

func TestSimManager(t *testing.T) {
	N := 9 // number of peers to generate

	allPeers = make([]*peer.Peer, N)

	mgrMap := make(map[peer.ID]*manager)
	for i := range allPeers {
		p := newPeer(fmt.Sprintf("%d", i))
		allPeers[i] = p.peer

		net := testNet{
			mgr:  mgrMap,
			loc:  p.local,
			self: p.peer,
			rand: p.rand,
		}
		mgrMap[p.local.ID()] = newManager(net, net.GetKnownPeers, p.log, &Parameters{SaltLifetime: 100 * time.Second})
	}

	// start all the managers
	for _, mgr := range mgrMap {
		mgr.start()
	}

	time.Sleep(6 * time.Second)

	for i, p := range allPeers {
		neighbors := mgrMap[p.ID()].getNeighbors()

		assert.NotEmpty(t, neighbors, "Peer %d has no neighbors", i)
		assert.Equal(t, removeDuplicatePeers(neighbors), neighbors, "Peer %d has non unique neighbors", i)
	}

	// close all the managers
	for _, mgr := range mgrMap {
		mgr.close()
	}
}
