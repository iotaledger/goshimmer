package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/identity"
)

var (
	_FPCInboundBytes  *uint64
	_FPCOutboundBytes *uint64

	previousNeighbors = make(map[identity.ID]gossipTrafficMetric)
	gossipOldTx       uint32
	gossipOldRx       uint32
)

func FPCInboundBytes() uint64 {
	return atomic.LoadUint64(_FPCInboundBytes)
}

func FPCOutboundBytes() uint64 {
	return atomic.LoadUint64(_FPCOutboundBytes)
}

type gossipTrafficMetric struct {
	BytesRead    uint32
	BytesWritten uint32
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
