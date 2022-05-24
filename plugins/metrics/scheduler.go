package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/syncutils"
)

var (
	// schedulerRate rate at which messages are scheduled.
	schedulerRate time.Duration

	// readyMessagesCount number of ready messages in the scheduler buffer.
	readyMessagesCount int

	// totalMessagesCount number of  messages in the scheduler buffer.
	totalMessagesCount int

	// bufferSize number of bytes waiting to be scheduled.
	bufferSize int

	// maxBufferSize maximum number of messages can be stored in the buffer.
	maxBufferSize int

	// nodeQueueSizes current size of each node's queue.
	nodeQueueSizes map[identity.ID]int
	// nodeQueueSizes current amount of aMana of each node in the queue.
	nodeAccessMana *schedulerutils.AccessManaCache
	// nodeQueueSizesMutex protect map from concurrent read/write.
	nodeQueueSizesMutex syncutils.RWMutex
)

func measureSchedulerMetrics() {
	nodeQueueSizesMutex.Lock()
	defer nodeQueueSizesMutex.Unlock()
	nodeQueueSizes = make(map[identity.ID]int)
	for k, v := range deps.Tangle.Scheduler.NodeQueueSizes() {
		nodeQueueSizes[k] = v
	}
	if nodeAccessMana == nil {
		nodeAccessMana = deps.Tangle.Scheduler.AccessManaCache()
	}
	bufferSize = deps.Tangle.Scheduler.BufferSize()
	maxBufferSize = deps.Tangle.Options.SchedulerParams.MaxBufferSize
	schedulerRate = deps.Tangle.Scheduler.Rate()
	readyMessagesCount = deps.Tangle.Scheduler.ReadyMessagesCount()
	totalMessagesCount = deps.Tangle.Scheduler.TotalMessagesCount()
}

// SchedulerNodeQueueSizes current size of each node's queue.
func SchedulerNodeQueueSizes() map[string]int {
	nodeQueueSizesMutex.RLock()
	defer nodeQueueSizesMutex.RUnlock()

	// copy the original map
	clone := make(map[string]int)
	for key, element := range nodeQueueSizes {
		clone[key.String()] = element
	}

	return clone
}

// SchedulerNodeAManaAmount current aMana value for each node in the queue.
func SchedulerNodeAManaAmount() map[string]float64 {
	nodeQueueSizesMutex.RLock()
	defer nodeQueueSizesMutex.RUnlock()

	// copy the original map
	clone := make(map[string]float64)
	for key := range nodeQueueSizes {
		clone[key.String()] = nodeAccessMana.GetCachedMana(key)
	}

	return clone
}

// SchedulerTotalBufferMessagesCount returns if the node is synced based on tangle time.
func SchedulerTotalBufferMessagesCount() int {
	return totalMessagesCount
}

// SchedulerReadyMessagesCount number of ready messages in the scheduler buffer.
func SchedulerReadyMessagesCount() int {
	return readyMessagesCount
}

// SchedulerMaxBufferSize returns the maximum buffer size.
func SchedulerMaxBufferSize() int {
	return maxBufferSize
}

// SchedulerBufferSize number of bytes waiting to be scheduled.
func SchedulerBufferSize() int {
	return bufferSize
}

// SchedulerRate rate at which messages are scheduled.
func SchedulerRate() int64 {
	return schedulerRate.Milliseconds()
}
