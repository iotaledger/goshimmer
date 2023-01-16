package dashboardmetrics

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/syncutils"
	"go.uber.org/atomic"
)

// BlockType defines the component for the different BPS metrics.
type BlockType byte

const (
	// DataBlock denotes data block type.
	DataBlock BlockType = iota
	// Transaction denotes transaction block.
	Transaction
)

// String returns the stringified component type.
func (c BlockType) String() string {
	switch c {
	case DataBlock:
		return "DataBlock"
	case Transaction:
		return "Transaction"
	default:
		return "Unknown"
	}
}

// ComponentType defines the component for the different BPS metrics.
type ComponentType byte

const (
	// Received denotes blocks received from the network.
	Received ComponentType = iota
	// Issued denotes blocks that the node itself issued.
	Issued
	// Allowed denotes blocks that passed the filter checks.
	Allowed
	// Attached denotes blocks stored by the block store.
	Attached
	// Solidified denotes blocks solidified by the solidifier.
	Solidified
	// Scheduled denotes blocks scheduled by the scheduler.
	Scheduled
	// SchedulerDropped denotes blocks dropped by the scheduler.
	SchedulerDropped
	// SchedulerSkipped denotes confirmed blocks skipped by the scheduler.
	SchedulerSkipped
	// Booked denotes blocks booked by the booker.
	Booked
)

// String returns the stringified component type.
func (c ComponentType) String() string {
	switch c {
	case Received:
		return "Received"
	case Issued:
		return "Issued"
	case Allowed:
		return "Allowed"
	case Attached:
		return "Attached"
	case Solidified:
		return "Solidified"
	case Scheduled:
		return "Scheduled"
	case SchedulerDropped:
		return "SchedulerDropped"
	case SchedulerSkipped:
		return "SchedulerSkipped"
	case Booked:
		return "Booked"
	default:
		return fmt.Sprintf("Unknown (%d)", c)
	}
}

// initial values at start of the node.
var (
	// number of solid blocks in the database at startup.
	initialBlockCountPerComponentDB = make(map[ComponentType]uint64)
)

// the same metrics as above, but since the start of a node.
var (
	// Number of blocks per component (store, scheduler, booker) type since start of the node.
	// One for dashboard (reset every time is read), other for grafana with cumulative value.
	blockCountPerComponentDashboard = make(map[ComponentType]uint64)
	blockCountPerComponentGrafana   = make(map[ComponentType]uint64)

	// protect map from concurrent read/write.
	blockCountPerComponentMutex syncutils.RWMutex

	// current number of missing blocks in missingBlockStorage.
	missingBlockCountDB atomic.Uint64

	// sum of times since issued (since start of the node).
	sumTimesSinceIssued = make(map[ComponentType]time.Duration)
	sumTimeMutex        syncutils.RWMutex
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
func BlockCountSinceStartPerComponentGrafana() map[ComponentType]uint64 {
	blockCountPerComponentMutex.RLock()
	defer blockCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range blockCountPerComponentGrafana {
		clone[key] = element
	}

	return clone
}

// InitialBlockCountPerComponentGrafana returns a map of block count per component types and their count at the start of the node.
func InitialBlockCountPerComponentGrafana() map[ComponentType]uint64 {
	blockCountPerComponentMutex.RLock()
	defer blockCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range initialBlockCountPerComponentDB {
		clone[key] = element
	}

	return clone
}

// BlockCountSinceStartPerComponentDashboard returns a map of block count per component types and their count since last time the value was read.
func BlockCountSinceStartPerComponentDashboard() map[ComponentType]uint64 {
	blockCountPerComponentMutex.RLock()
	defer blockCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range blockCountPerComponentDashboard {
		clone[key] = element
	}

	return clone
}

// BlockRequestQueueSize returns the number of block requests the node currently has registered.
func BlockRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

func increasePerComponentCounter(c ComponentType) {
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
