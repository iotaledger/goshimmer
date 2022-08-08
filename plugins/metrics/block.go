package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/core/syncutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
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
	// Store denotes blocks stored by the block store.
	Store ComponentType = iota
	// Solidifier denotes blocks solidified by the solidifier.
	Solidifier
	// Scheduler denotes blocks scheduled by the scheduler.
	Scheduler
	// SchedulerDropped denotes blocks dropped by the scheduler.
	SchedulerDropped
	// SchedulerSkipped denotes confirmed blocks skipped by the scheduler.
	SchedulerSkipped
	// Booker denotes blocks booked by the booker.
	Booker
)

// String returns the stringified component type.
func (c ComponentType) String() string {
	switch c {
	case Store:
		return "Store"
	case Solidifier:
		return "Solidifier"
	case Scheduler:
		return "Scheduler"
	case SchedulerDropped:
		return "SchedulerDropped"
	case SchedulerSkipped:
		return "SchedulerSkipped"
	case Booker:
		return "booker"
	default:
		return "Unknown"
	}
}

// initial values at start of the node.
var (
	// number of solid blocks in the database at startup.
	initialBlockCountPerComponentDB = make(map[ComponentType]uint64)

	// initial number of missing blocks in missingBlockStorage (at startup).
	initialMissingBlockCountDB uint64

	// helper variable that is only calculated at init phase. unit is milliseconds!
	initialSumTimeSinceReceived = make(map[ComponentType]time.Duration)

	// sum of time blocks spend in the queue (since start of the node).
	initialSumSchedulerBookedTime time.Duration
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

	// sum of times since received (since start of the node).
	sumTimesSinceReceived = make(map[ComponentType]time.Duration)

	// sum of times since issued (since start of the node).
	sumTimesSinceIssued = make(map[ComponentType]time.Duration)
	sumTimeMutex        syncutils.RWMutex

	// sum of time blocks spend in the queue (since start of the node).
	sumSchedulerBookedTime time.Duration
	schedulerTimeMutex     syncutils.RWMutex
)

// other metrics stored since the start of a node.
var (
	// current number of finalized blocks.
	finalizedBlockCount      = make(map[BlockType]uint64)
	finalizedBlockCountMutex syncutils.RWMutex

	// total time it took all blocks to finalize after being received. unit is milliseconds!
	blockFinalizationReceivedTotalTime = make(map[BlockType]uint64)

	// total time it took all blocks to finalize after being issued. unit is milliseconds!
	blockFinalizationIssuedTotalTime = make(map[BlockType]uint64)
	blockFinalizationTotalTimeMutex  syncutils.RWMutex

	// current number of block tips.
	blockTips atomic.Uint64

	// total number of parents of all blocks per parent type.
	parentsCountPerType      = make(map[tangleold.ParentsType]uint64)
	parentsCountPerTypeMutex syncutils.RWMutex

	// Number of blocks per payload type since start of the node.
	blockCountPerPayload      = make(map[payload.Type]uint64)
	blockCountPerPayloadMutex syncutils.RWMutex

	// number of blocks being requested by the block layer.
	requestQueueSize atomic.Int64

	// number of blocks being requested by the block layer.
	solidificationRequests atomic.Uint64

	// counter for the received BPS (for dashboard).
	mpsReceivedSinceLastMeasurement atomic.Uint64
)

// //// Exported functions to obtain metrics from outside //////

// BlockCountSinceStartPerPayload returns a map of block payload types and their count since the start of the node.
func BlockCountSinceStartPerPayload() map[payload.Type]uint64 {
	blockCountPerPayloadMutex.RLock()
	defer blockCountPerPayloadMutex.RUnlock()

	// copy the original map
	clone := make(map[payload.Type]uint64)
	for key, element := range blockCountPerPayload {
		clone[key] = element
	}

	return clone
}

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

// BlockTips returns the actual number of tips in the block tangle.
func BlockTips() uint64 {
	return blockTips.Load()
}

// SolidificationRequests returns the number of solidification requests since start of node.
func SolidificationRequests() uint64 {
	return solidificationRequests.Load()
}

// BlockRequestQueueSize returns the number of block requests the node currently has registered.
func BlockRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

// SumTimeSinceReceived returns the cumulative time it took for all block to be processed by each component since being received since startup [milliseconds].
func SumTimeSinceReceived() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range sumTimesSinceReceived {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// InitialSumTimeSinceReceived returns the cumulative time it took for all block to be processed by each component since being received at startup [milliseconds].
func InitialSumTimeSinceReceived() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range initialSumTimeSinceReceived {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// SumTimeSinceIssued returns the cumulative time it took for all block to be processed by each component since being issued [milliseconds].
func SumTimeSinceIssued() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range sumTimesSinceIssued {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// SchedulerTime returns the cumulative time it took for all block to become scheduled before startup [milliseconds].
func SchedulerTime() (result int64) {
	schedulerTimeMutex.RLock()
	defer schedulerTimeMutex.RUnlock()
	result = sumSchedulerBookedTime.Milliseconds()
	return
}

// InitialSchedulerTime returns the cumulative time it took for all block to become scheduled at startup [milliseconds].
func InitialSchedulerTime() (result int64) {
	schedulerTimeMutex.RLock()
	defer schedulerTimeMutex.RUnlock()
	result = initialSumSchedulerBookedTime.Milliseconds()
	return
}

// InitialBlockMissingCountDB returns the number of blocks in missingBlockStore at startup.
func InitialBlockMissingCountDB() uint64 {
	return initialMissingBlockCountDB
}

// BlockMissingCountDB returns the number of blocks in missingBlockStore.
func BlockMissingCountDB() uint64 {
	return missingBlockCountDB.Load()
}

// BlockFinalizationTotalTimeSinceReceivedPerType returns total time block received it took for all blocks to finalize per block type.
func BlockFinalizationTotalTimeSinceReceivedPerType() map[BlockType]uint64 {
	blockFinalizationTotalTimeMutex.RLock()
	defer blockFinalizationTotalTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[BlockType]uint64)
	for key, element := range blockFinalizationReceivedTotalTime {
		clone[key] = element
	}

	return clone
}

// BlockFinalizationTotalTimeSinceIssuedPerType returns total time since block issuance it took for all blocks to finalize per block type.
func BlockFinalizationTotalTimeSinceIssuedPerType() map[BlockType]uint64 {
	blockFinalizationTotalTimeMutex.RLock()
	defer blockFinalizationTotalTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[BlockType]uint64)
	for key, element := range blockFinalizationIssuedTotalTime {
		clone[key] = element
	}

	return clone
}

// FinalizedBlockCountPerType returns the number of blocks finalized per block type.
func FinalizedBlockCountPerType() map[BlockType]uint64 {
	finalizedBlockCountMutex.RLock()
	defer finalizedBlockCountMutex.RUnlock()

	// copy the original map
	clone := make(map[BlockType]uint64)
	for key, element := range finalizedBlockCount {
		clone[key] = element
	}

	return clone
}

// ParentCountPerType returns a map of parent counts per parent type.
func ParentCountPerType() map[tangleold.ParentsType]uint64 {
	parentsCountPerTypeMutex.RLock()
	defer parentsCountPerTypeMutex.RUnlock()

	// copy the original map
	clone := make(map[tangleold.ParentsType]uint64)
	for key, element := range parentsCountPerType {
		clone[key] = element
	}

	return clone
}

// //// Handling data updates and measuring.
func increasePerPayloadCounter(p payload.Type) {
	blockCountPerPayloadMutex.Lock()
	defer blockCountPerPayloadMutex.Unlock()

	// increase cumulative metrics
	blockCountPerPayload[p]++
}

func increasePerComponentCounter(c ComponentType) {
	blockCountPerComponentMutex.Lock()
	defer blockCountPerComponentMutex.Unlock()

	// increase cumulative metrics
	blockCountPerComponentDashboard[c]++
	blockCountPerComponentGrafana[c]++
}

func increasePerParentType(c tangleold.ParentsType) {
	parentsCountPerTypeMutex.Lock()
	defer parentsCountPerTypeMutex.Unlock()

	// increase cumulative metrics
	parentsCountPerType[c]++
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

func measureBlockTips() {
	blockTips.Store(uint64(deps.Tangle.TipManager.TipCount()))
}

// increases the received BPS counter
func increaseReceivedBPSCounter() {
	mpsReceivedSinceLastMeasurement.Inc()
}

// measures the received BPS value
func measureReceivedBPS() {
	// sample the current counter value into a measured BPS value
	sampledBPS := mpsReceivedSinceLastMeasurement.Load()

	// reset the counter
	mpsReceivedSinceLastMeasurement.Store(0)

	// trigger events for outside listeners
	Events.ReceivedBPSUpdated.Trigger(&ReceivedBPSUpdatedEvent{BPS: sampledBPS})
}

func measureRequestQueueSize() {
	size := int64(deps.Tangle.Requester.RequestQueueSize())
	requestQueueSize.Store(size)
}

func measureInitialDBStats() {
	dbStatsResult := deps.Tangle.Storage.DBStats()

	initialBlockCountPerComponentDB[Store] = uint64(dbStatsResult.StoredCount)
	initialBlockCountPerComponentDB[Solidifier] = uint64(dbStatsResult.SolidCount)
	initialBlockCountPerComponentDB[Booker] = uint64(dbStatsResult.BookedCount)
	initialBlockCountPerComponentDB[Scheduler] = uint64(dbStatsResult.ScheduledCount)

	initialSumTimeSinceReceived[Solidifier] = dbStatsResult.SumSolidificationReceivedTime
	initialSumTimeSinceReceived[Booker] = dbStatsResult.SumBookedReceivedTime
	initialSumTimeSinceReceived[Scheduler] = dbStatsResult.SumSchedulerReceivedTime

	initialSumSchedulerBookedTime = dbStatsResult.SumSchedulerBookedTime

	initialMissingBlockCountDB = uint64(dbStatsResult.MissingBlockCount)
}
