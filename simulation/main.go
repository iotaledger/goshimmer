package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/selection"
	"github.com/wollac/autopeering/server"
	"github.com/wollac/autopeering/simulation/visualizer"
	"github.com/wollac/autopeering/transport"
)

var (
	allPeers       []*peer.Peer
	protocolMap    = make(map[peer.ID]*selection.Protocol)
	idMap          = make(map[peer.ID]uint16)
	status         = NewStatusMap() // key: timestamp, value: Status
	neighborhoods  = make(map[peer.ID][]*peer.Peer)
	Links          = []Link{}
	termTickerChan = make(chan bool)
	incomingChan   = make(chan *selection.PeeringEvent, 10)
	outgoingChan   = make(chan *selection.PeeringEvent, 10)
	dropChan       = make(chan *selection.DroppedEvent, 10)
	closing        = make(chan struct{})
	RecordConv     = NewConvergenceList()
	StartTime      time.Time
	wg             sync.WaitGroup

	N            = 100
	vEnabled     = false
	SimDuration  = 300
	SaltLifetime = 300 * time.Second
	DropAllFlag  = false
)

// dummyDiscovery is a dummy implementation of DiscoveryProtocol never returning any verified peers.
type dummyDiscovery struct{}

func (d dummyDiscovery) IsVerified(p *peer.Peer) bool   { return true }
func (d dummyDiscovery) EnsureVerified(p *peer.Peer)    {}
func (d dummyDiscovery) GetVerifiedPeers() []*peer.Peer { return allPeers }

func RunSim() {
	allPeers = make([]*peer.Peer, N)

	network := transport.NewNetwork()
	serverMap := make(map[peer.ID]*server.Server, N)
	disc := dummyDiscovery{}

	// subscribe to the events
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(e *selection.PeeringEvent) { incomingChan <- e }))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(e *selection.PeeringEvent) { outgoingChan <- e }))
	selection.Events.Dropped.Attach(events.NewClosure(func(e *selection.DroppedEvent) { dropChan <- e }))

	//lambda := (float64(N) / SaltLifetime.Seconds()) * 10
	initialSalt := 0.

	log.Println("Creating peers...")
	for i := range allPeers {
		name := fmt.Sprintf("%d", i)
		network.AddTransport(name)

		peer := newPeer(name, (time.Duration(initialSalt) * time.Second))
		allPeers[i] = peer.peer

		id := peer.local.ID()
		idMap[id] = uint16(i)

		cfg := selection.Config{Log: peer.log,
			SaltLifetime:  SaltLifetime,
			DropNeighbors: DropAllFlag,
		}
		protocol := selection.New(peer.local, disc, cfg)
		serverMap[id] = server.Listen(peer.local, network.GetTransport(name), peer.log, protocol)

		protocolMap[id] = protocol

		if vEnabled {
			visualizer.AddNode(id.String())
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
		srv := serverMap[peer.ID()]
		protocolMap[peer.ID()].Start(srv)
	}

	time.Sleep(time.Duration(SimDuration) * time.Second)
	// Stop updating visualizer
	if vEnabled {
		termTickerChan <- true
	}

	// Stop simulation
	log.Println("Closing...")
	for _, peer := range allPeers {
		protocolMap[peer.ID()].Close()
	}
	log.Println("Closing Done")
	close(closing)

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

			// handle incoming peering requests
			case req := <-incomingChan:
				from := idMap[req.Peer.ID()]
				to := idMap[req.Self]
				status.Append(from, to, INCOMING)

			// handle outgoing peering requests
			case req := <-outgoingChan:
				from := idMap[req.Self]
				to := idMap[req.Peer.ID()]
				status.Append(from, to, OUTBOUND)

				// accepted/rejected is only recorded for outgoing requests
				if len(req.Services) > 0 {
					status.Append(from, to, ACCEPTED)
					Links = append(Links, NewLink(from, to, time.Since(StartTime).Milliseconds()))
					if vEnabled {
						visualizer.AddLink(req.Self.String(), req.Peer.ID().String())
					}
				} else {
					status.Append(from, to, REJECTED)
				}

			// handle dropped peers incoming and outgoing
			case req := <-dropChan:
				from := idMap[req.Self]
				to := idMap[req.DroppedID]
				status.Append(from, to, DROPPED)
				DropLink(from, to, time.Since(StartTime).Milliseconds(), Links)
				if vEnabled {
					visualizer.RemoveLink(req.Self.String(), req.DroppedID.String())
				}

			case <-ticker.C:
				updateConvergence(time.Since(StartTime))

			case <-closing:
				return
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
	for _, prot := range protocolMap {
		l := len(prot.GetNeighbors())
		if l == 8 {
			counter++
		}
		avgNeighbors += l
	}
	c := (float64(counter) / float64(N)) * 100
	avg := float64(avgNeighbors) / float64(N)
	RecordConv.Append(Convergence{time, c, avg})
}
