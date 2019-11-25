package selection

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/salt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
	allPeers []*peer.Peer
)

type testPeer struct {
	local *peer.Local
	peer  *peer.Peer
	db    peer.DB
	log   *zap.SugaredLogger
	rand  *rand.Rand // random number generator
}

func newPeer(name string) testPeer {
	var l *zap.Logger
	var err error
	if name == "1" {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewDevelopment() //zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger := l.Sugar()
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, _ := peer.NewLocal(db)
	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)
	// add a dummy service
	local.Services()[name] = peer.NetworkAddress{}
	p := peer.NewPeer(local.PublicKey(), name)
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

func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt, services peer.ServiceMap) (peer.ServiceMap, error) {
	return n.mgr[p.ID()].acceptRequest(n.self, s, services), nil
}

func (n testNet) GetKnownPeers() []*peer.Peer {
	list := make([]*peer.Peer, len(allPeers)-1)
	i := 0
	for _, peer := range allPeers {
		if peer.ID() == n.self.ID() {
			continue
		}

		list[i] = peer
		i++
	}
	return list
}

func TestSimManager(t *testing.T) {
	N := 9 // number of peers to generate

	allPeers = make([]*peer.Peer, N)

	mgrMap := make(map[peer.ID]*manager)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i))
		allPeers[i] = peer.peer

		net := testNet{
			mgr:  mgrMap,
			loc:  peer.local,
			self: peer.peer,
			rand: peer.rand,
		}
		mgrMap[peer.local.ID()] = newManager(net, net.GetKnownPeers, peer.log, &Parameters{SaltLifetime: 100 * time.Second})
	}

	// start all the managers
	for _, mgr := range mgrMap {
		mgr.start()
	}

	time.Sleep(5 * time.Second)

	for i, peer := range allPeers {
		neighbors := mgrMap[peer.ID()].getNeighbors()

		assert.NotEmpty(t, neighbors, "Peer %d has no neighbors", i)
		assert.Equal(t, removeDuplicatePeers(neighbors), neighbors, "Peer %d has non unique neighbors", i)
	}

	// close all the managers
	for _, mgr := range mgrMap {
		mgr.close()
	}
}
