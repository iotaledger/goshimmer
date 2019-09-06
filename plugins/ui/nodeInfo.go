package ui

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
)

var start = time.Now()

var receivedTpsCounter uint64
var solidTpsCounter uint64

var tpsQueue []uint32

const maxQueueSize int = 3600

type nodeInfo struct {
	ID                string   `json:"id"`
	ChosenNeighbors   []string `json:"chosenNeighbors"`
	AcceptedNeighbors []string `json:"acceptedNeighbors"`
	KnownPeersSize    int      `json:"knownPeers"`
	NeighborhoodSize  int      `json:"neighborhood"`
	Uptime            uint64   `json:"uptime"`
	ReceivedTps       uint64   `json:"receivedTps"`
	SolidTps          uint64   `json:"solidTps"`
}

func gatherInfo() nodeInfo {

	// update tps queue
	tpsQueue = append(tpsQueue, uint32(receivedTpsCounter))
	if len(tpsQueue) > maxQueueSize {
		tpsQueue = tpsQueue[1:]
	}

	// update neighbors
	chosenNeighbors := []string{}
	acceptedNeighbors := []string{}
	for _, peer := range chosenneighbors.INSTANCE.Peers.GetMap() {
		chosenNeighbors = append(chosenNeighbors, peer.String())
	}
	for _, peer := range acceptedneighbors.INSTANCE.Peers.GetMap() {
		acceptedNeighbors = append(acceptedNeighbors, peer.String())
	}

	receivedTps, solidTps := updateTpsCounters()
	duration := time.Since(start) / time.Second
	info := nodeInfo{
		ID:                accountability.OwnId().StringIdentifier,
		ChosenNeighbors:   chosenNeighbors,
		AcceptedNeighbors: acceptedNeighbors,
		KnownPeersSize:    knownpeers.INSTANCE.Peers.Len(),
		NeighborhoodSize:  neighborhood.INSTANCE.Peers.Len(),
		Uptime:            uint64(duration),
		ReceivedTps:       receivedTps,
		SolidTps:          solidTps,
	}
	return info
}

func updateTpsCounters() (uint64, uint64) {
	receivedTps := atomic.LoadUint64(&receivedTpsCounter)
	solidTps := atomic.LoadUint64(&solidTpsCounter)
	atomic.StoreUint64(&receivedTpsCounter, 0)
	atomic.StoreUint64(&solidTpsCounter, 0)
	return receivedTps, solidTps
}
