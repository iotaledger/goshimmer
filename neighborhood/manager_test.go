package neighborhood

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	mgrMap[A.local.ID()].Run()
	mgrMap[B.local.ID()].Run()
	mgrMap[C.local.ID()].Run()

	time.Sleep(3 * time.Second)

	neighborhoodA := mgrMap[A.local.ID()].GetNeighbors()
	neighborhoodB := mgrMap[B.local.ID()].GetNeighbors()
	neighborhoodC := mgrMap[C.local.ID()].GetNeighbors()

	log.Println("A", neighborhoodA)
	log.Println("B", neighborhoodB)
	log.Println("C", neighborhoodC)

	assert.Equal(t, sliceUniqMap(neighborhoodA), neighborhoodA, "A Neighbors")
	assert.Equal(t, sliceUniqMap(neighborhoodB), neighborhoodB, "B Neighbors")
	assert.Equal(t, sliceUniqMap(neighborhoodC), neighborhoodC, "A Neighbors")
}

func sliceUniqMap(s []*peer.Peer) []*peer.Peer {
	seen := make(map[*peer.Peer]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}
