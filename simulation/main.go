package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/awalterschulze/gographviz"
	"github.com/wollac/autopeering/neighborhood"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

var (
	allPeers []*peer.Peer
	idMap    = make(map[peer.ID]int)
	results  []result
)

type result struct {
	request  int
	accepted int
	incoming int
	rejected int
	dropped  int
}

type testPeer struct {
	local *peer.Local
	peer  *peer.Peer
	db    *peer.DB
	log   *zap.SugaredLogger
	rand  *rand.Rand // random number generator
}

func newPeer(name string, i int) testPeer {
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
	s, _ := salt.NewSalt(time.Duration(25+i) * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(time.Duration(25+i) * time.Second)
	local.SetPublicSalt(s)
	p := peer.NewPeer(local.PublicKey(), name)
	return testPeer{local, p, db, log, rand.New(rand.NewSource(time.Now().UnixNano()))}
}

type testNet struct {
	neighborhood.Network
	mgr   map[peer.ID]*neighborhood.Manager
	local *peer.Local
	self  *peer.Peer
	rand  *rand.Rand
}

func (n testNet) DropPeer(p *peer.Peer) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	results[idMap[p.ID()]].dropped++
	n.mgr[p.ID()].DropNeighbor(n.self.ID())
}

func (n testNet) Local() *peer.Local {
	return n.local
}
func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	results[idMap[n.self.ID()]].request++
	results[idMap[p.ID()]].incoming++
	response := n.mgr[p.ID()].AcceptRequest(n.self, s)
	if response {
		results[idMap[n.self.ID()]].accepted++
	} else {
		results[idMap[n.self.ID()]].rejected++
	}
	return response, nil
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

func RunSim() {
	N := 100
	allPeers = make([]*peer.Peer, N)
	results = make([]result, N)
	mgrMap := make(map[peer.ID]*neighborhood.Manager)
	neighborhoods := make(map[peer.ID][]*peer.Peer)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i), i)
		allPeers[i] = peer.peer
		net := testNet{
			mgr:   mgrMap,
			local: peer.local,
			self:  peer.peer,
			rand:  peer.rand,
		}
		idMap[peer.local.ID()] = i
		mgrMap[peer.local.ID()] = neighborhood.NewManager(net, net.GetKnownPeers, peer.log)
	}

	for _, peer := range allPeers {
		mgrMap[peer.ID()].Run()
	}

	time.Sleep(20 * time.Second)
	log.Println("resetting measures")
	for _, peer := range allPeers {
		results[idMap[peer.ID()]].request = 0
		results[idMap[peer.ID()]].accepted = 0
		results[idMap[peer.ID()]].rejected = 0
		results[idMap[peer.ID()]].incoming = 0
		results[idMap[peer.ID()]].dropped = 0
	}
	time.Sleep(30 * time.Minute)

	list := []Edge{}
	g := gographviz.NewGraph()
	if err := g.SetName("G"); err != nil {
		panic(err)
	}
	if err := g.SetDir(true); err != nil {
		panic(err)
	}

	avgResult := result{}

	l := 0.
	fmt.Printf("\nID\tOUT\tACC\tREJ\tIN\tDROP\n")
	for _, peer := range allPeers {
		neighborhoods[peer.ID()] = mgrMap[peer.ID()].GetNeighbors()
		//log.Println(idMap[peer.ID()], "(", len(mgrMap[peer.ID()].GetOutbound().GetPeers()), ",", len(mgrMap[peer.ID()].GetInbound().GetPeers()), ")")

		fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\n", idMap[peer.ID()], results[idMap[peer.ID()]].request, results[idMap[peer.ID()]].accepted, results[idMap[peer.ID()]].rejected, results[idMap[peer.ID()]].incoming, results[idMap[peer.ID()]].dropped)
		avgResult.request += results[idMap[peer.ID()]].request
		avgResult.accepted += results[idMap[peer.ID()]].accepted
		avgResult.rejected += results[idMap[peer.ID()]].rejected
		avgResult.incoming += results[idMap[peer.ID()]].incoming
		avgResult.dropped += results[idMap[peer.ID()]].dropped

		// add a new vertex
		if err := g.AddNode("G", fmt.Sprintf("%d", idMap[peer.ID()]), nil); err != nil {
			panic(err)
		}

		l += float64(len(neighborhoods[peer.ID()]))

		for _, ng := range neighborhoods[peer.ID()] {
			//log.Printf(" %d ", idMap[ng.ID()])
			edge := NewEdge(idMap[peer.ID()], idMap[ng.ID()])
			if !HasEdge(edge, list) {
				list = append(list, edge)
			}
		}
	}
	fmt.Println("Average")
	fmt.Printf("\nOUT\t\tACC\t\tREJ\t\tIN\t\tDROP\n")
	fmt.Printf("%v\t%v\t%v\t%v\t%v\n", float64(avgResult.request)/float64(N), float64(avgResult.accepted)/float64(N), float64(avgResult.rejected)/float64(N), float64(avgResult.incoming)/float64(N), float64(avgResult.dropped)/float64(N))

	log.Println("Average len/edges: ", l/float64(N), len(list))

	for _, edge := range list {
		if err := g.AddEdge(fmt.Sprintf("%d", edge.X), fmt.Sprintf("%d", edge.Y), true, nil); err != nil {
			panic(err)
		}
	}

	s := g.String()
	fmt.Println(s)

}

type Edge struct {
	X, Y int
}

func NewEdge(x, y int) Edge {
	return Edge{x, y}
}

func HasEdge(target Edge, list []Edge) bool {
	for _, edge := range list {
		if (edge.X == target.X && edge.Y == target.Y) ||
			(edge.X == target.Y && edge.Y == target.X) {
			return true
		}
	}
	return false
}

func main() {
	RunSim()
}
