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

////// Handling data updates and measuring //////

func increasePerPayloadCounter(p payload.Type) {
	messageCountPerPayloadMutex.Lock()
	defer messageCountPerPayloadMutex.Unlock()

	// increase cumulative metrics
	messageCountPerPayload[p]++
	messageTotalCount.Inc()
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.TipSelector.TipCount()))
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
