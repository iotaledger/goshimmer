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

	// Current number of solid messages in the node's database.
	messageSolidCount atomic.Uint64

	messageTotalCountDB atomic.Uint64

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

// MessageTotalCount returns the total number of messages seen since the start of the node.
func MessageTotalCount() uint64 {
	return messageTotalCount.Load()
}

// MessageCountPerPayload returns a map of message payload types and their count since the start of the node.
func MessageCountPerPayload() map[payload.Type]uint64 {
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

// MessageSolidCount returns the number of messages that are solid.
func MessageSolidCount() uint64 {
	return messageSolidCount.Load()
}

// MessageTotalCountDB returns the number of messages that are stored in the DB.
func MessageTotalCountDB() uint64 {
	return messageTotalCountDB.Load()
}

// AvgSolidificationTime returns the average time it takes for a message to become solid.
func AvgSolidificationTime() float64 {
	return avgSolidificationTime.Load()
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

// ReceivedMessagesPerSecond retrieves the current messages per second number.
func ReceivedMessagesPerSecond() uint64 {
	return measuredReceivedMPS.Load()
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
	messageSolidCount.Store(uint64(solid))
	messageTotalCountDB.Store(uint64(total))
	avgSolidificationTime.Store(avgSolidTime)
}
