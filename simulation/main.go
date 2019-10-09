package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/selection"
	"github.com/wollac/autopeering/simulation/visualizer"
)

var (
	allPeers       []*peer.Peer
	mgrMap         = make(map[peer.ID]*selection.Manager)
	idMap          = make(map[peer.ID]uint16)
	status         = NewStatusMap() // key: timestamp, value: Status
	neighborhoods  = make(map[peer.ID][]*peer.Peer)
	Links          = []Link{}
	linkChan       = make(chan Event, 100)
	termTickerChan = make(chan bool)
	RecordConv     = NewConvergenceList()
	StartTime      time.Time
	wg             sync.WaitGroup
	wgClose        sync.WaitGroup

	N            = 100
	vEnabled     = false
	SimDuration  = 300
	SaltLifetime = 300 * time.Second
	DropAllFlag  = false
)

func RunSim() {
	allPeers = make([]*peer.Peer, N)
	initialSalt := 0.
	//lambda := (float64(N) / SaltLifetime.Seconds()) * 10
	for i := range allPeers {
		peer := newPeer(fmt.Sprintf("%d", i), (time.Duration(initialSalt) * time.Second))
		allPeers[i] = peer.peer
		net := simNet{
			mgr:  mgrMap,
			loc:  peer.local,
			self: peer.peer,
			rand: peer.rand,
		}
		idMap[peer.local.ID()] = uint16(i)
		mgrMap[peer.local.ID()] = selection.NewManager(net, SaltLifetime, net.GetKnownPeers, peer.log, DropAllFlag)

		if vEnabled {
			visualizer.AddNode(peer.local.ID().String())
		}

		// initialSalt = initialSalt + (1 / lambda)				 // constant rate
		// initialSalt = initialSalt + rand.ExpFloat64()/lambda  // poisson process
		initialSalt = rand.Float64() * SaltLifetime.Seconds() // random
	}

	fmt.Println("start link analysis")
	runLinkAnalysis()

	if vEnabled {
		statVisualizer()
	}

	StartTime = time.Now()
	for _, peer := range allPeers {
		mgrMap[peer.ID()].Start()
	}
	wgClose.Add(len(allPeers))

	time.Sleep(time.Duration(SimDuration) * time.Second)
	// Stop updating visualizer
	if vEnabled {
		termTickerChan <- true
	}
	// Stop simulation
	for _, p := range allPeers {
		p := p
		go func(p *peer.Peer) {
			mgrMap[p.ID()].Close()
			wgClose.Done()
		}(p)
	}
	log.Println("Closing...")
	wgClose.Wait()
	log.Println("Closing Done")
	linkChan <- Event{TERMINATE, 0, 0, 0}

	// Wait until analysis goroutine stops
	wg.Wait()

	// Start finalize simulation result
	linkAnalysis := linksToString(LinkSurvival(Links))
	err := writeCSV(linkAnalysis, "linkAnalysis", []string{"X", "Y"})
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	//	log.Println(linkAnalysis)

	convAnalysis := convergenceToString(RecordConv.convergence)
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

	log.Println("Simulation Done")
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
	fmt.Println("start sim")
	RunSim()
}

func runLinkAnalysis() {
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case newEvent := <-linkChan:
				switch newEvent.eType {
				case ESTABLISHED:
					Links = append(Links, NewLink(newEvent.x, newEvent.y, newEvent.timestamp.Milliseconds()))
					//log.Println("New Link", newEvent)
				case DROPPED:
					DropLink(newEvent.x, newEvent.y, newEvent.timestamp.Milliseconds(), Links)
					//log.Println("Link Dropped", newEvent)
				case TERMINATE:
					return
				}
			case <-ticker.C:
				updateConvergence(time.Since(StartTime))
			}
		}
	}()
}

func statVisualizer() {
	wg.Add(1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		defer wg.Done()
		for {
			select {
			case <-termTickerChan:
				return
			case <-ticker.C:
				visualizer.UpdateConvergence(RecordConv.GetConvergence())
				visualizer.UpdateAvgNeighbors(RecordConv.GetAvgNeighbors())
			}
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
	RecordConv.Append(Convergence{time, c, avg})
}
