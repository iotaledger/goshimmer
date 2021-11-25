package metrics

import (
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

var (
	previousNeighbors = make(map[identity.ID]gossipTrafficMetric)
	gossipOldTx       uint64
	gossipOldRx       uint64
	gossipCurrentTx   atomic.Uint64
	gossipCurrentRx   atomic.Uint64

	analysisOutboundBytes atomic.Uint64
)

// GossipInboundPackets returns the total amount of inbound gossip packets.
func GossipInboundPackets() uint64 {
	return gossipCurrentRx.Load()
}

// GossipOutboundPackets returns the total amount of outbound gossip packets.
func GossipOutboundPackets() uint64 {
	return gossipCurrentTx.Load()
}

// AnalysisOutboundBytes returns the total outbound analysis traffic.
func AnalysisOutboundBytes() uint64 {
	return analysisOutboundBytes.Load()
}

func measureGossipTraffic() {
	g := gossipCurrentTraffic()
	gossipCurrentRx.Store(g.PacketsRead)
	gossipCurrentTx.Store(g.PacketsWritten)
}

type gossipTrafficMetric struct {
	PacketsRead    uint64
	PacketsWritten uint64
}

func gossipCurrentTraffic() (g gossipTrafficMetric) {
	neighbors := deps.GossipMgr.AllNeighbors()

	currentNeighbors := make(map[identity.ID]bool)
	for _, neighbor := range neighbors {
		currentNeighbors[neighbor.ID()] = true

		if _, ok := previousNeighbors[neighbor.ID()]; !ok {
			previousNeighbors[neighbor.ID()] = gossipTrafficMetric{
				PacketsRead:    neighbor.PacketsRead(),
				PacketsWritten: neighbor.PacketsWritten(),
			}
		}

		g.PacketsRead += neighbor.PacketsRead()
		g.PacketsWritten += neighbor.PacketsWritten()
	}

	for prevNeighbor := range previousNeighbors {
		if _, ok := currentNeighbors[prevNeighbor]; !ok {
			gossipOldRx += previousNeighbors[prevNeighbor].PacketsRead
			gossipOldTx += previousNeighbors[prevNeighbor].PacketsWritten
			delete(currentNeighbors, prevNeighbor)
		}
	}

	g.PacketsRead += gossipOldRx
	g.PacketsWritten += gossipOldTx

	return
}
