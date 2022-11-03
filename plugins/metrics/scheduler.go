package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/syncutils"
)

var (
	// schedulerRate rate at which blocks are scheduled.
	schedulerRate time.Duration

	// readyBlocksCount number of ready blocks in the scheduler buffer.
	readyBlocksCount int

	// totalBlocksCount number of  blocks in the scheduler buffer.
	totalBlocksCount int

	// bufferSize number of bytes waiting to be scheduled.
	bufferSize int

	// schedulerDeficit deficit value
	schedulerDeficit float64
	// maxBufferSize maximum number of blocks can be stored in the buffer.
	maxBufferSize int

	// issuerQueueSizes current size of each issuer's queue.
	issuerQueueSizes map[identity.ID]int
	// issuerQueueSizesMutex protect map from concurrent read/write.
	issuerQueueSizesMutex syncutils.RWMutex
)

func measureSchedulerMetrics() {
	issuerQueueSizesMutex.Lock()
	defer issuerQueueSizesMutex.Unlock()
	issuerQueueSizes = make(map[identity.ID]int)
	for k, v := range deps.Protocol.CongestionControl.Scheduler().IssuerQueueSizes() {
		issuerQueueSizes[k] = v
	}
	bufferSize = deps.Protocol.CongestionControl.Scheduler().BufferSize()
	maxBufferSize = deps.Protocol.CongestionControl.Scheduler().MaxBufferSize()
	schedulerDeficit, _ = deps.Protocol.CongestionControl.Scheduler().Deficit(deps.Local.ID()).Float64()
	schedulerRate = deps.Protocol.CongestionControl.Scheduler().Rate()
	readyBlocksCount = deps.Protocol.CongestionControl.Scheduler().ReadyBlocksCount()
	totalBlocksCount = deps.Protocol.CongestionControl.Scheduler().TotalBlocksCount()
}

// SchedulerIssuerQueueSizes current size of each issuer's queue.
func SchedulerIssuerQueueSizes() map[string]int {
	issuerQueueSizesMutex.RLock()
	defer issuerQueueSizesMutex.RUnlock()

	// copy the original map
	clone := make(map[string]int)
	for key, element := range issuerQueueSizes {
		clone[key.String()] = element
	}

	return clone
}

// SchedulerIssuerAManaAmount current aMana value for each issuer in the queue.
func SchedulerIssuerAManaAmount() map[string]int64 {
	issuerQueueSizesMutex.RLock()
	defer issuerQueueSizesMutex.RUnlock()

	// copy the original map
	clone := make(map[string]int64)
	for key := range issuerQueueSizes {
		clone[key.String()], _ = deps.Protocol.Engine().ManaTracker.Mana(key)
	}

	return clone
}

// SchedulerTotalBufferBlocksCount returns if the issuer is synced based on tangle time.
func SchedulerTotalBufferBlocksCount() int {
	return totalBlocksCount
}

// SchedulerReadyBlocksCount number of ready blocks in the scheduler buffer.
func SchedulerReadyBlocksCount() int {
	return readyBlocksCount
}

// SchedulerMaxBufferSize returns the maximum buffer size.
func SchedulerMaxBufferSize() int {
	return maxBufferSize
}

// SchedulerDeficit local issuer's deficit value.
func SchedulerDeficit() float64 {
	return schedulerDeficit
}

// SchedulerBufferSize number of bytes waiting to be scheduled.
func SchedulerBufferSize() int {
	return bufferSize
}

// SchedulerRate rate at which blocks are scheduled.
func SchedulerRate() int64 {
	return schedulerRate.Milliseconds()
}
