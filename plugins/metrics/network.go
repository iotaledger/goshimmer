package metrics

import (
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

var (
	_FPCInboundBytes  atomic.Uint64
	_FPCOutboundBytes atomic.Uint64

	previousNeighbors = make(map[identity.ID]gossipTrafficMetric)
	gossipOldTx       uint64
	gossipOldRx       uint64
	gossipCurrentTx   atomic.Uint64
	gossipCurrentRx   atomic.Uint64

	analysisOutboundBytes atomic.Uint64
)

// FPCInboundBytes returns the total inbound FPC traffic.
func FPCInboundBytes() uint64 {
	return _FPCInboundBytes.Load()
}

// FPCOutboundBytes returns the total outbound FPC traffic.
func FPCOutboundBytes() uint64 {
	return _FPCOutboundBytes.Load()
}

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
	neighbors := gossip.Manager().AllNeighbors()

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
