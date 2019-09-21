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
	status   = NewStatusMap() // key: timestamp, value: Status
)

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
	status.Append(idMap[p.ID()], idMap[n.self.ID()], DROPPED)
	n.mgr[p.ID()].DropNeighbor(n.self.ID())
}

func (n testNet) Local() *peer.Local {
	return n.local
}
func (n testNet) RequestPeering(p *peer.Peer, s *salt.Salt) (bool, error) {
	//time.Sleep(time.Duration(n.rand.Intn(max-min+1)+min) * time.Microsecond)
	from := idMap[n.self.ID()]
	to := idMap[p.ID()]
	status.Append(from, to, OUTBOUND)
	status.Append(to, from, INCOMING)
	response := n.mgr[p.ID()].AcceptRequest(n.self, s)
	if response {
		status.Append(from, to, ACCEPTED)
	} else {
		status.Append(from, to, REJECTED)
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
	mgrMap := make(map[peer.ID]*neighborhood.Manager)
	neighborhoods := make(map[peer.ID][]*peer.Peer)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i), 1000)
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

	list := []Edge{}
	g := gographviz.NewGraph()
	if err := g.SetName("G"); err != nil {
		panic(err)
	}
	if err := g.SetDir(true); err != nil {
		panic(err)
	}

	avgResult := StatusSum{}

	l := 0.
	fmt.Printf("\nID\tOUT\tACC\tREJ\tIN\tDROP\n")
	for _, peer := range allPeers {
		neighborhoods[peer.ID()] = mgrMap[peer.ID()].GetNeighbors()

		summary := status.GetSummary(idMap[peer.ID()])
		fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\n", idMap[peer.ID()], summary.outbound, summary.accepted, summary.rejected, summary.incoming, summary.dropped)

		avgResult.outbound += summary.outbound
		avgResult.accepted += summary.accepted
		avgResult.rejected += summary.rejected
		avgResult.incoming += summary.incoming
		avgResult.dropped += summary.dropped

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
	fmt.Printf("%v\t%v\t%v\t%v\t%v\n", float64(avgResult.outbound)/float64(N), float64(avgResult.accepted)/float64(N), float64(avgResult.rejected)/float64(N), float64(avgResult.incoming)/float64(N), float64(avgResult.dropped)/float64(N))

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
