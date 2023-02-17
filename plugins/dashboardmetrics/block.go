package dashboardmetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

// the same metrics as above, but since the start of a node.
var (
	// Number of blocks per component (store, scheduler, booker) type since start of the node.
	// One for dashboard (reset every time is read), other for grafana with cumulative value.
	blockCountPerComponentDashboard = make(map[collector.ComponentType]uint64)
	blockCountPerComponentGrafana   = make(map[collector.ComponentType]uint64)

	// protect map from concurrent read/write.
	blockCountPerComponentMutex syncutils.RWMutex
)

// other metrics stored since the start of a node.
var (
	// number of blocks being requested by the block layer.
	requestQueueSize atomic.Int64

	// counter for the received BPS (for dashboard).
	mpsAttachedSinceLastMeasurement atomic.Uint64
)

// //// Exported functions to obtain metrics from outside //////

// BlockCountSinceStartPerComponentGrafana returns a map of block count per component types and their count since the start of the node.
func BlockCountSinceStartPerComponentGrafana() map[collector.ComponentType]uint64 {
	blockCountPerComponentMutex.RLock()
	defer blockCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[collector.ComponentType]uint64)
	for key, element := range blockCountPerComponentGrafana {
		clone[key] = element
	}

	return clone
}

// BlockCountSinceStartPerComponentDashboard returns a map of block count per component types and their count since last time the value was read.
func BlockCountSinceStartPerComponentDashboard() map[collector.ComponentType]uint64 {
	blockCountPerComponentMutex.RLock()
	defer blockCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[collector.ComponentType]uint64)
	for key, element := range blockCountPerComponentDashboard {
		clone[key] = element
	}

	return clone
}

// BlockRequestQueueSize returns the number of block requests the node currently has registered.
func BlockRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

func increasePerComponentCounter(c collector.ComponentType) {
	blockCountPerComponentMutex.Lock()
	defer blockCountPerComponentMutex.Unlock()

	// increase cumulative metrics
	blockCountPerComponentDashboard[c]++
	blockCountPerComponentGrafana[c]++
}

// measures the Component Counter value per second.
func measurePerComponentCounter() {
	// sample the current counter value into a measured BPS value
	componentCounters := BlockCountSinceStartPerComponentDashboard()

	// reset the counter
	blockCountPerComponentMutex.Lock()
	for key := range blockCountPerComponentDashboard {
		blockCountPerComponentDashboard[key] = 0
	}
	blockCountPerComponentMutex.Unlock()

	// trigger events for outside listeners
	Events.ComponentCounterUpdated.Trigger(&ComponentCounterUpdatedEvent{ComponentStatus: componentCounters})
}

// increases the received BPS counter.
func increaseReceivedBPSCounter() {
	mpsAttachedSinceLastMeasurement.Inc()
}

// measures the received BPS value.
func measureAttachedBPS() {
	// sample the current counter value into a measured BPS value
	sampledBPS := mpsAttachedSinceLastMeasurement.Load()

	// reset the counter
	mpsAttachedSinceLastMeasurement.Store(0)

	// trigger events for outside listeners
	Events.AttachedBPSUpdated.Trigger(&AttachedBPSUpdatedEvent{BPS: sampledBPS})
}

func measureRequestQueueSize() {
	size := int64(deps.Protocol.Engine().BlockRequester.QueueSize())
	requestQueueSize.Store(size)
}
