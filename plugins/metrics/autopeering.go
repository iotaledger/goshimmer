package metrics

import (
	"sync"
	"time"
)

var (
	neighborDropCount           uint64
	neighborConnectionsLifeTime time.Duration
	neighborMutex               sync.RWMutex
)

func NeighborDropCount() uint64 {
	neighborMutex.RLock()
	defer neighborMutex.RUnlock()
	return neighborDropCount
}

func AvgNeighborConnectionLifeTime() float64 {
	neighborMutex.RLock()
	defer neighborMutex.RUnlock()
	if neighborDropCount == 0 {
		return 0.
	}
	return float64(neighborConnectionsLifeTime.Milliseconds()) / float64(neighborDropCount)
}
