package metrics

import (
	"sync"
	"time"

	gossipPkg "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/events"
)

var (
	neighborDropCount           uint64
	neighborConnectionsLifeTime time.Duration
	neighborMutex               sync.RWMutex

	connectionsCount uint64
	sumDistance      uint64
	minDistance      = uint64(^uint32(0))
	maxDistance      uint64
	distanceMutex    sync.RWMutex
)

var (
	onNeighborRemoved = events.NewClosure(func(n *gossipPkg.Neighbor) {
		neighborMutex.Lock()
		defer neighborMutex.Unlock()
		neighborDropCount++
		neighborConnectionsLifeTime += time.Since(n.ConnectionEstablished())
	})

	onAutopeeringSelection = events.NewClosure(func(ev *selection.PeeringEvent) {
		distanceMutex.Lock()
		defer distanceMutex.Unlock()
		connectionsCount++
		distance := uint64(ev.Distance)
		if distance < minDistance {
			minDistance = distance
		}
		if distance > maxDistance {
			maxDistance = distance
		}
		sumDistance += distance
	})
)

// NeighborDropCount returns the neighbor drop count.
func NeighborDropCount() uint64 {
	neighborMutex.RLock()
	defer neighborMutex.RUnlock()
	return neighborDropCount
}

// AvgNeighborConnectionLifeTime return the average neighbor connection lifetime.
func AvgNeighborConnectionLifeTime() float64 {
	neighborMutex.RLock()
	defer neighborMutex.RUnlock()
	if neighborDropCount == 0 {
		return 0.
	}
	return float64(neighborConnectionsLifeTime.Milliseconds()) / float64(neighborDropCount)
}

// ConnectionsCount returns the neighbors connections count.
func ConnectionsCount() uint64 {
	distanceMutex.RLock()
	defer distanceMutex.RUnlock()
	return connectionsCount
}

// AutopeeringDistanceStats returns statistics of the autopeering distance function.
func AutopeeringDistanceStats() (min, max uint64, avg float64) {
	distanceMutex.RLock()
	defer distanceMutex.RUnlock()
	min, max = minDistance, maxDistance
	if connectionsCount == 0 {
		avg = 0
		return
	}
	avg = float64(sumDistance) / float64(connectionsCount)
	return
}
