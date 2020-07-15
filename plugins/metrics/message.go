package metrics

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/atomic"
)

var (
	// Total number of processed messages since start of the node.
	messageTotalCount atomic.Uint64

	// current number of solid messages in the node's database
	messageSolidCountDBIter atomic.Uint64

	// current number of solid messages in the node's database
	messageSolidCountDBInc atomic.Uint64

	// number of solid messages in the database at startup
	initialMessageSolidCountDB atomic.Uint64

	// current number of messages in the node's database
	messageTotalCountDBIter atomic.Uint64

	// current number of messages in the node's database
	messageTotalCountDBInc atomic.Uint64

	// number of messages in the database at startup
	initialMessageTotalCountDB atomic.Uint64

	// average time it takes to solidify a message
	avgSolidificationTime atomic.Float64

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
	copy := make(map[payload.Type]uint64)
	for key, element := range messageCountPerPayload {
		copy[key] = element
	}

	return copy
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return messageTips.Load()
}

// MessageRequestQueueSize returns the number of message requests the node currently has registered.
func MessageRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

// MessageSolidCountIter returns the number of messages that are solid.
func MessageSolidCountIter() uint64 {
	return messageSolidCountDBIter.Load()
}

// MessageTotalCountDBIter returns the number of messages that are stored in the DB.
func MessageTotalCountDBIter() uint64 {
	return messageTotalCountDBIter.Load()
}

// MessageSolidCountInc returns the number of messages that are solid.
func MessageSolidCountInc() uint64 {
	return initialMessageSolidCountDB.Load() + messageSolidCountDBInc.Load()
}

// MessageTotalCountDBInc returns the number of messages that are stored in the DB.
func MessageTotalCountDBInc() uint64 {
	return initialMessageTotalCountDB.Load() + messageTotalCountDBInc.Load()
}

// AvgSolidificationTime returns the average time it takes for a message to become solid.
func AvgSolidificationTime() float64 {
	return avgSolidificationTime.Load()
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
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.TipSelector().TipCount()))
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
	size := int64(messagelayer.MessageRequester().RequestQueueSize())
	requestQueueSize.Store(size)
}

func measureDBStats() {
	solid, total, avgSolidTime := messagelayer.Tangle().DBStats()
	messageSolidCountDBIter.Store(uint64(solid))
	messageTotalCountDBIter.Store(uint64(total))
	avgSolidificationTime.Store(avgSolidTime)
}

func measureInitialDBStats() {
	solid, total, avgSolidTime := messagelayer.Tangle().DBStats()
	initialMessageSolidCountDB.Store(uint64(solid))
	initialMessageTotalCountDB.Store(uint64(total))
	avgSolidificationTime.Store(avgSolidTime)
}
