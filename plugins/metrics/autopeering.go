package metrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/generics/event"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/node/p2p"
)

var (
	neighborDropCount           uint64
	neighborConnectionsLifeTime time.Duration
	neighborMutex               sync.RWMutex

	neighborConnectionsCount    atomic.Uint64
	autopeeringConnectionsCount uint64
	sumDistance                 uint64
	minDistance                 = uint64(^uint32(0))
	maxDistance                 uint64
	distanceMutex               sync.RWMutex
)

var (
	onNeighborRemoved = event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
		neighborMutex.Lock()
		defer neighborMutex.Unlock()
		neighborDropCount++
		neighborConnectionsLifeTime += time.Since(event.Neighbor.ConnectionEstablished())
	})

	onNeighborAdded = event.NewClosure(func(_ *p2p.NeighborAddedEvent) {
		neighborConnectionsCount.Inc()
	})

	onAutopeeringSelection = event.NewClosure(func(event *selection.PeeringEvent) {
		distanceMutex.Lock()
		defer distanceMutex.Unlock()
		autopeeringConnectionsCount++
		distance := uint64(event.Distance)
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

// NeighborConnectionsCount returns the neighbors connections count.
func NeighborConnectionsCount() uint64 {
	return neighborConnectionsCount.Load()
}

// AutopeeringDistanceStats returns statistics of the autopeering distance function.
func AutopeeringDistanceStats() (min, max uint64, avg float64) {
	distanceMutex.RLock()
	defer distanceMutex.RUnlock()
	min, max = minDistance, maxDistance
	if autopeeringConnectionsCount == 0 {
		avg = 0
		return
	}
	avg = float64(sumDistance) / float64(autopeeringConnectionsCount)
	return
}
