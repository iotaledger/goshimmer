package neighborhood

import (
	"log"
	"testing"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

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
	network
	mgr   map[peer.ID]*Manager
	local *peer.Local
	self  *peer.Peer
}

func (n testNet) DropPeer(p *peer.Peer) {
	n.mgr[p.ID()].DropNeighbor(n.self)
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

func TestManager(t *testing.T) {
	mgrMap := make(map[peer.ID]*Manager)

	A := newPeer("A")
	B := newPeer("B")
	C := newPeer("C")
	netA := testNet{
		mgr:   mgrMap,
		local: A.local,
		self:  A.peer,
	}
	netB := testNet{
		mgr:   mgrMap,
		local: B.local,
		self:  B.peer,
	}
	netC := testNet{
		mgr:   mgrMap,
		local: C.local,
		self:  C.peer,
	}

	allPeers = []*peer.Peer{A.peer, B.peer, C.peer}

	mgrMap[A.local.ID()] = NewManager(netA, netA.GetKnownPeers, A.log)
	mgrMap[B.local.ID()] = NewManager(netB, netB.GetKnownPeers, B.log)
	mgrMap[C.local.ID()] = NewManager(netC, netC.GetKnownPeers, C.log)

	time.Sleep(10 * time.Second)

	log.Println("A", mgrMap[A.local.ID()].inbound.GetPeers(), mgrMap[A.local.ID()].outbound.GetPeers())
	log.Println("B", mgrMap[B.local.ID()].inbound.GetPeers(), mgrMap[B.local.ID()].outbound.GetPeers())
	log.Println("C", mgrMap[C.local.ID()].inbound.GetPeers(), mgrMap[C.local.ID()].outbound.GetPeers())
}
