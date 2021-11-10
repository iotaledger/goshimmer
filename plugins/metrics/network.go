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

// GossipInboundBytes returns the total inbound gossip traffic.
func GossipInboundBytes() uint64 {
	return gossipCurrentRx.Load()
}

// GossipOutboundBytes returns the total outbound gossip traffic.
func GossipOutboundBytes() uint64 {
	return gossipCurrentTx.Load()
}

// AnalysisOutboundBytes returns the total outbound analysis traffic.
func AnalysisOutboundBytes() uint64 {
	return analysisOutboundBytes.Load()
}

func measureGossipTraffic() {
	g := gossipCurrentTraffic()
	gossipCurrentRx.Store(g.BytesRead)
	gossipCurrentTx.Store(g.BytesWritten)
}

type gossipTrafficMetric struct {
	BytesRead    uint64
	BytesWritten uint64
}

func gossipCurrentTraffic() (g gossipTrafficMetric) {
	neighbors := deps.GossipMgr.AllNeighbors()

	currentNeighbors := make(map[identity.ID]bool)
	for _, neighbor := range neighbors {
		currentNeighbors[neighbor.ID()] = true

		if _, ok := previousNeighbors[neighbor.ID()]; !ok {
			previousNeighbors[neighbor.ID()] = gossipTrafficMetric{
				BytesRead:    neighbor.BytesRead(),
				BytesWritten: neighbor.BytesWritten(),
			}
		}

		g.BytesRead += neighbor.BytesRead()
		g.BytesWritten += neighbor.BytesWritten()
	}

	for prevNeighbor := range previousNeighbors {
		if _, ok := currentNeighbors[prevNeighbor]; !ok {
			gossipOldRx += previousNeighbors[prevNeighbor].BytesRead
			gossipOldTx += previousNeighbors[prevNeighbor].BytesWritten
			delete(currentNeighbors, prevNeighbor)
		}
	}

	g.BytesRead += gossipOldRx
	g.BytesWritten += gossipOldTx

	return
}
