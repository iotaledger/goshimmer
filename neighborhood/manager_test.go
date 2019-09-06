package neighborhood

import (
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

var (
	allPeers []*peer.Peer
	idMap    = make(map[peer.ID]int)
)

type testPeer struct {
	local *peer.Local
	peer  *peer.Peer
	db    *peer.DB
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
	priv, _ := peer.GeneratePrivateKey()
	db := peer.NewMapDB(log.Named("db"))
	local := peer.NewLocal(priv, db)
	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)
	p := peer.NewPeer(local.PublicKey(), name)
	return testPeer{local, p, db, log, rand.New(rand.NewSource(time.Now().UnixNano()))}
}

type testNet struct {
	Network
	mgr   map[peer.ID]*Manager
	local *peer.Local
	self  *peer.Peer
	rand  *rand.Rand
}

func (n testNet) DropPeer(p *peer.Peer) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	n.mgr[p.ID()].DropNeighbor(n.self.ID())
}

func (n testNet) Local() *peer.Local {
	return n.local
}
func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
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

func TestSimManager(t *testing.T) {
	N := 9
	allPeers = make([]*peer.Peer, N)
	mgrMap := make(map[peer.ID]*Manager)
	neighborhoods := make(map[peer.ID][]*peer.Peer)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i))
		allPeers[i] = peer.peer
		net := testNet{
			mgr:   mgrMap,
			local: peer.local,
			self:  peer.peer,
			rand:  peer.rand,
		}
		idMap[peer.local.ID()] = i
		mgrMap[peer.local.ID()] = NewManager(net, net.GetKnownPeers, peer.log)
	}

	for _, p := range allPeers {
		d := peer.SortBySalt(p.ID().Bytes(), mgrMap[p.ID()].net.Local().GetPublicSalt().GetBytes(), mgrMap[p.ID()].getKnownPeers())
		log.Println("\n", p.ID())
		for _, dist := range d {
			log.Println(dist.Remote.ID())
		}
	}

	for _, p := range allPeers {
		d := peer.SortBySalt(p.ID().Bytes(), mgrMap[p.ID()].net.Local().GetPrivateSalt().GetBytes(), mgrMap[p.ID()].getKnownPeers())
		log.Println("\n", p.ID())
		for _, dist := range d {
			log.Println(dist.Remote.ID())
		}
	}

	// d := peer.SortBySalt(allPeers[1].ID().Bytes(), mgrMap[allPeers[1].ID()].net.Local().GetPublicSalt().GetBytes(), mgrMap[allPeers[1].ID()].getKnownPeers())
	// log.Println("\n", allPeers[1].ID())
	// for _, dist := range d {
	// 	log.Println(dist.Remote.ID(), dist.Distance)
	// }

	// d = peer.SortBySalt(allPeers[1].ID().Bytes(), mgrMap[allPeers[1].ID()].net.Local().GetPrivateSalt().GetBytes(), mgrMap[allPeers[1].ID()].getKnownPeers())
	// log.Println("\n", allPeers[1].ID())
	// for _, dist := range d {
	// 	log.Println(dist.Remote.ID(), dist.Distance)
	// }

	for _, peer := range allPeers {
		mgrMap[peer.ID()].Run()
	}

	time.Sleep(5 * time.Second)

	for i, peer := range allPeers {
		neighborhoods[peer.ID()] = mgrMap[peer.ID()].GetNeighbors()
		log.Println(idMap[peer.ID()], "(", len(mgrMap[peer.ID()].outbound.GetPeers()), ",", len(mgrMap[peer.ID()].inbound.GetPeers()), ")")
		for _, ng := range neighborhoods[peer.ID()] {
			log.Printf(" %d ", idMap[ng.ID()])
		}

		assert.Equal(t, sliceUniqMap(neighborhoods[peer.ID()]), neighborhoods[peer.ID()], fmt.Sprintln("Peer: ", i))
		//assert.Equal(t, N-1, len(neighborhoods[peer.ID()]), fmt.Sprintln("Peer: ", i))
	}

}
