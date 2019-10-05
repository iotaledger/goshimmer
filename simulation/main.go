package main

import (
	"fmt"
	"log"
	"time"

	"github.com/wollac/autopeering/neighborhood"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/simulation/visualizer"
)

var (
	allPeers      []*peer.Peer
	mgrMap        = make(map[peer.ID]*neighborhood.Manager)
	idMap         = make(map[peer.ID]uint16)
	status        = NewStatusMap() // key: timestamp, value: Status
	neighborhoods = make(map[peer.ID][]*peer.Peer)
	Links         []Link
	linkChan      = make(chan Event, 100)
	RecordConv    []Convergence
	StartTime     time.Time

	N            = 100
	vEnabled     = false
	SimDuration  = 300
	SaltLifetime = 300 * time.Second
)

func RunSim() {
	allPeers = make([]*peer.Peer, N)
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i), uint16(i))
		allPeers[i] = peer.peer
		net := simNet{
			mgr:  mgrMap,
			loc:  peer.local,
			self: peer.peer,
			rand: peer.rand,
		}
		idMap[peer.local.ID()] = uint16(i)
		mgrMap[peer.local.ID()] = neighborhood.NewManager(net, SaltLifetime, net.GetKnownPeers, peer.log)

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
		mgrMap[peer.ID()].Start()
	}

	time.Sleep(time.Duration(SimDuration) * time.Second)

	log.Println("Len:", len(Links))
	//log.Println(Links)

	linkAnalysis := linksToString(LinkSurvival(Links))
	err := writeCSV(linkAnalysis, "linkAnalysis", []string{"X", "Y"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	//log.Println(linkAnalysis)

	convAnalysis := convergenceToString(RecordConv)
	err = writeCSV(convAnalysis, "convAnalysis", []string{"X", "Y"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	//log.Println(RecordConv)

	msgAnalysis := messagesToString(status)
	err = writeCSV(msgAnalysis, "msgAnalysis", []string{"ID", "OUT", "ACC", "REJ", "IN", "DROP"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}

}

func main() {
	p := parseInput("input.txt")
	setParam(p)

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
