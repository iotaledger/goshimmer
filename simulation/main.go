package main

import (
	"fmt"
	"log"
	"time"

	"github.com/awalterschulze/gographviz"
	"github.com/wollac/autopeering/neighborhood"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/simulation/visualizer"
)

var (
	allPeers   []*peer.Peer
	mgrMap     = make(map[peer.ID]*neighborhood.Manager)
	idMap      = make(map[peer.ID]uint16)
	status     = NewStatusMap() // key: timestamp, value: Status
	Links      = []Link{}
	linkChan   = make(chan Event, 100)
	RecordConv = []Convergence{}
	StartTime  time.Time

	N            = 100
	vEnabled     = true
	SimDuration  = 300
	SaltLifetime = 300 * time.Second
)

func RunSim() {
	allPeers = make([]*peer.Peer, N)
	neighborhoods := make(map[peer.ID][]*peer.Peer)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i), uint16(i))
		allPeers[i] = peer.peer
		net := testNet{
			mgr:   mgrMap,
			local: peer.local,
			self:  peer.peer,
			rand:  peer.rand,
		}
		idMap[peer.local.ID()] = uint16(i)
		conf := neighborhood.Config{
			Log:           peer.log,
			GetKnownPeers: net.GetKnownPeers,
			Lifetime:      SaltLifetime,
		}
		mgrMap[peer.local.ID()] = neighborhood.NewManager(net, conf)

		if vEnabled {
			visualizer.AddNode(peer.local.ID().String())
		}
	}

	runLinkAnalysis()

	if vEnabled {
		statVisualizer()
	}

	StartTime = time.Now()
	for _, peer := range allPeers {
		mgrMap[peer.ID()].Run()
	}

	time.Sleep(time.Duration(SimDuration) * time.Second)

	log.Println("Len:", len(Links))
	//log.Println(Links)

	linkAnalysis := linksToString(LinkSurvival((Links)))
	err := writeCSV(linkAnalysis, "linkAnalysis", []string{"X", "Y"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	log.Println(linkAnalysis)

	convAnalysis := convergenceToString(RecordConv)
	err = writeCSV(convAnalysis, "convAnalysis", []string{"X", "Y"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	log.Println(RecordConv)

	list := []Link{}
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

		fmt.Println()
		// add a new vertex
		if err := g.AddNode("G", fmt.Sprintf("%d", idMap[peer.ID()]), nil); err != nil {
			panic(err)
		}

		l += float64(len(neighborhoods[peer.ID()]))

		// for _, ng := range neighborhoods[peer.ID()] {
		// 	//log.Printf(" %d ", idMap[ng.ID()])
		// 	link := NewLink(idMap[peer.ID()], idMap[ng.ID()], 0)
		// 	if !HasLink(link, list) {
		// 		list = append(list, link)
		// 	}
		// }
	}
	fmt.Println("Average")
	fmt.Printf("\nOUT\t\tACC\t\tREJ\t\tIN\t\tDROP\n")
	fmt.Printf("%v\t%v\t%v\t%v\t%v\n", float64(avgResult.outbound)/float64(N), float64(avgResult.accepted)/float64(N), float64(avgResult.rejected)/float64(N), float64(avgResult.incoming)/float64(N), float64(avgResult.dropped)/float64(N))

	log.Println("Average len/edges: ", l/float64(N), len(list))

	// for _, link := range list {
	// 	if err := g.AddEdge(fmt.Sprintf("%d", link.x), fmt.Sprintf("%d", link.y), true, nil); err != nil {
	// 		panic(err)
	// 	}
	// }

	// s := g.String()
	// fmt.Println(s)

}

func main() {
	var s *visualizer.Server
	if vEnabled {
		s = visualizer.NewServer()
		go s.Run()
		<-s.Start
	}
	RunSim()
}

func runLinkAnalysis() {
	go func() {

		for newEvent := range linkChan {
			switch newEvent.eType {
			case ESTABLISHED:
				Links = append(Links, NewLink(newEvent.x, newEvent.y, newEvent.timestamp.Milliseconds()))
				//log.Println("New Link", newEvent)
			case DROPPED:
				DropLink(newEvent.x, newEvent.y, newEvent.timestamp.Milliseconds(), Links)

				//log.Println("Link Dropped", newEvent)
			}
			updateConvergence(newEvent.timestamp)
		}
	}()
}

func statVisualizer() {
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		for range ticker.C {
			visualizer.UpdateConvergence(getConvergence())
			visualizer.UpdateAvgNeighbors(getAvgNeighbors())
		}
	}()
}

func updateConvergence(time time.Duration) {
	counter := 0
	avgNeighbors := 0
	for _, peer := range mgrMap {
		l := len(peer.GetNeighbors())
		if l == 8 {
			counter++
		}
		avgNeighbors += l
	}
	c := (float64(counter) / float64(N)) * 100
	avg := float64(avgNeighbors) / float64(N)
	RecordConv = append(RecordConv, Convergence{time, c, avg})
}

func getConvergence() float64 {
	if len(RecordConv) > 0 {
		return RecordConv[len(RecordConv)-1].counter
	}
	return 0
}

func getAvgNeighbors() float64 {
	if len(RecordConv) > 0 {
		return RecordConv[len(RecordConv)-1].avgNeighbors
	}
	return 0
}
