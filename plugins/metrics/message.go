package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/atomic"
)

var (
	// Total number of processed messages since start of the node.
	messageTotalCount atomic.Uint64

	// number of messages in the database at startup
	initialMessageTotalCountDB uint64

	// current number of messages in the node's database
	messageTotalCountDB atomic.Uint64

	// number of solid messages in the database at startup
	initialMessageSolidCountDB uint64

	// current number of solid messages in the node's database
	messageSolidCountDBInc atomic.Uint64

	// helper variable that is only calculated at init phase. unit is milliseconds!
	initialSumSolidificationTime float64

	// sum of solidification time (since start of the node)
	sumSolidificationTime time.Duration
	solidTimeMutex        syncutils.RWMutex

	// initial number of missing messages in missingMessageStorage (at startup)
	initialMissingMessageCountDB uint64

	// current number of missing messages in missingMessageStorage
	missingMessageCountDB atomic.Uint64

	// current number of message tips.
	messageTips atomic.Uint64

	// counter for the received MPS
	mpsReceivedSinceLastMeasurement atomic.Uint64

	// measured value of the received MPS
	measuredReceivedMPS atomic.Uint64

	// Number of messages per payload type since start of the node.
	messageCountPerPayload = make(map[payload.Type]uint64)

	// protect map from concurrent read/write.
	messageCountPerPayloadMutex syncutils.RWMutex

	// number of messages being requested by the message layer.
	requestQueueSize atomic.Int64
)

////// Exported functions to obtain metrics from outside //////

// MessageTotalCountSinceStart returns the total number of messages seen since the start of the node.
func MessageTotalCountSinceStart() uint64 {
	return messageTotalCount.Load()
}

// MessageCountSinceStartPerPayload returns a map of message payload types and their count since the start of the node.
func MessageCountSinceStartPerPayload() map[payload.Type]uint64 {
	messageCountPerPayloadMutex.RLock()
	defer messageCountPerPayloadMutex.RUnlock()

	// copy the original map
	clone := make(map[payload.Type]uint64)
	for key, element := range messageCountPerPayload {
		clone[key] = element
	}

	return clone
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return messageTips.Load()
}

// MessageRequestQueueSize returns the number of message requests the node currently has registered.
func MessageRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

// MessageSolidCountDB returns the number of messages that are solid in the DB.
func MessageSolidCountDB() uint64 {
	return initialMessageSolidCountDB + messageSolidCountDBInc.Load()
}

// MessageTotalCountDB returns the number of messages that are stored in the DB.
func MessageTotalCountDB() uint64 {
	return initialMessageTotalCountDB + messageTotalCountDB.Load()
}

// AvgSolidificationTime returns the average time it takes for a message to become solid. [milliseconds]
func AvgSolidificationTime() (result float64) {
	solidTimeMutex.RLock()
	defer solidTimeMutex.RUnlock()
	totalSolid := MessageSolidCountDB()
	if totalSolid > 0 {
		result = (initialSumSolidificationTime + float64(sumSolidificationTime.Milliseconds())) / float64(totalSolid)
	}
	return
}

// MessageMissingCountDB returns the number of messages in missingMessageStore.
func MessageMissingCountDB() uint64 {
	return initialMissingMessageCountDB + missingMessageCountDB.Load()
}

// ReceivedMessagesPerSecond retrieves the current messages per second number.
func ReceivedMessagesPerSecond() uint64 {
	return measuredReceivedMPS.Load()
}

////// Handling data updates and measuring //////

func increasePerPayloadCounter(p payload.Type) {
	messageCountPerPayloadMutex.Lock()
	defer messageCountPerPayloadMutex.Unlock()

	// increase cumulative metrics
	messageCountPerPayload[p]++
	messageTotalCount.Inc()
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.Tangle().TipManager.TipCount()))
}

// increases the received MPS counter
func increaseReceivedMPSCounter() {
	mpsReceivedSinceLastMeasurement.Inc()
}

// measures the received MPS value
func measureReceivedMPS() {
	// sample the current counter value into a measured MPS value
	sampledMPS := mpsReceivedSinceLastMeasurement.Load()

	// store the measured value
	measuredReceivedMPS.Store(sampledMPS)

	// reset the counter
	mpsReceivedSinceLastMeasurement.Store(0)

	// trigger events for outside listeners
	Events.ReceivedMPSUpdated.Trigger(sampledMPS)
}

func measureRequestQueueSize() {
	size := int64(messagelayer.Tangle().Requester.RequestQueueSize())
	requestQueueSize.Store(size)
}

func measureInitialDBStats() {
	solid, total, avgSolidTime, missing := messagelayer.Tangle().Storage.DBStats()
	initialMessageSolidCountDB = uint64(solid)
	initialMessageTotalCountDB = uint64(total)
	initialSumSolidificationTime = avgSolidTime * float64(solid)
	initialMissingMessageCountDB = uint64(missing)
}
