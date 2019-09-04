package main

import (
	"fmt"
	"log"
	"time"

	"github.com/wollac/autopeering/neighborhood"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

func main() {
	N := 3
	allPeers = make([]*peer.Peer, N)
	mgrMap := make(map[peer.ID]*neighborhood.Manager)
	neighborhoods := make(map[peer.ID][]*peer.Peer)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i))
		allPeers[i] = peer.peer
		net := testNet{
			mgr:   mgrMap,
			local: peer.local,
			self:  peer.peer,
		}
		mgrMap[peer.local.ID()] = neighborhood.NewManager(net, net.GetKnownPeers, peer.log)
	}

	// for _, p := range allPeers {
	// 	d := peer.SortBySalt(p.ID().Bytes(), mgrMap[p.ID()].net.Local().GetPublicSalt().GetBytes(), allPeers)
	// 	log.Println("\n", p.ID())
	// 	for _, dist := range d {
	// 		log.Println(dist.Distance)
	// 	}
	// }

	for _, peer := range allPeers {
		mgrMap[peer.ID()].Run()
	}

	time.Sleep(2 * time.Second)

	for _, peer := range allPeers {
		neighborhoods[peer.ID()] = mgrMap[peer.ID()].GetNeighbors()
		log.Println(peer.ID(), neighborhoods[peer.ID()])
	}
}

var (
	allPeers []*peer.Peer
)

type testPeer struct {
	local *peer.Local
	peer  *peer.Peer
	db    *peer.DB
	log   *zap.SugaredLogger
}

func newPeer(name string) testPeer {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger := l.Sugar()
	log := logger.Named(name)
	priv, _ := peer.GeneratePrivateKey()
	db := peer.NewMapDB(log.Named("db"))
	local := peer.NewLocal(priv, db)
	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)
	p := peer.NewPeer(local.PublicKey(), name)
	return testPeer{local, p, db, log}
}

type testNet struct {
	neighborhood.Network
	mgr   map[peer.ID]*neighborhood.Manager
	local *peer.Local
	self  *peer.Peer
}

func (n testNet) DropPeer(p *peer.Peer) {
	n.mgr[p.ID()].DropNeighbor(n.self.ID())
}

func (n testNet) Local() *peer.Local {
	return n.local
}
func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	return n.mgr[p.ID()].AcceptRequest(n.self, s), nil
}

func (n testNet) GetKnownPeers() []*peer.Peer {
	list := make([]*peer.Peer, len(allPeers)-1)
	i := 0
	for _, peer := range allPeers {
		if peer != n.self {
			list[i] = peer
			i++
		}
	}
	return list
}
