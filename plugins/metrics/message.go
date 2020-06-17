package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/syncutils"
)

// counter for the received MPS
var mpsReceivedSinceLastMeasurement uint64

// measured value of the received MPS
var measuredReceivedMPS uint64

// Total number of processed messages since start of the node.
var messageTotalCount uint64

// current number of message tips
var messageTips uint64

// MPS figures for different payload types
var mpsPerPayloadSinceLastMeasurement = make(map[payload.Type]uint64)
var mpsPerPayloadMeasured = make(map[payload.Type]uint64)

// Number of messages per payload type since start of the node.
var messageCountPerPayload = make(map[payload.Type]uint64)

// protect maps from concurrent read/write
var messageLock syncutils.RWMutex

////// Exported functions to obtain metrics from outside //////

// MessageTotalCount returns the total number of messages seen since the start of the node.
func MessageTotalCount() uint64 {
	return atomic.LoadUint64(&messageTotalCount)
}

// MPS retrieves the current messages per second number.
func MPS() uint64 {
	return atomic.LoadUint64(&measuredReceivedMPS)
}

// MessageCountPerPayload returns a map of message payload types and their count since the start of the node.
func MessageCountPerPayload() map[payload.Type]uint64 {
	messageLock.RLock()
	defer messageLock.RUnlock()

	// copy the original map
	target := make(map[payload.Type]uint64)
	for key, element := range messageCountPerPayload {
		target[key] = element
	}

	return target
}

// MPSPerPayload returns a map of message payload types and their corresponding MPS values.
func MPSPerPayload() map[payload.Type]uint64 {
	messageLock.RLock()
	defer messageLock.RUnlock()

	// copy the original map
	target := make(map[payload.Type]uint64)
	for key, element := range mpsPerPayloadMeasured {
		target[key] = element
	}

	return target
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return atomic.LoadUint64(&messageTips)
}

////// Handling data updates and measuring //////

// increases the received MPS counter
func increaseReceivedMPSCounter() {
	atomic.AddUint64(&mpsReceivedSinceLastMeasurement, 1)
}

// measures the received MPS value
func measureReceivedMPS() {
	// sample the current counter value into a measured MPS value
	sampledMPS := atomic.LoadUint64(&mpsReceivedSinceLastMeasurement)

	// store the measured value
	atomic.StoreUint64(&measuredReceivedMPS, sampledMPS)

	// reset the counter
	atomic.StoreUint64(&mpsReceivedSinceLastMeasurement, 0)

	// trigger events for outside listeners
	Events.ReceivedMPSUpdated.Trigger(sampledMPS)
}

func increasePerPayloadMPSCounter(p payload.Type) {
	messageLock.Lock()
	defer messageLock.Unlock()
	// init counter with zero value if we see the payload for the first time
	if _, exist := mpsPerPayloadSinceLastMeasurement[p]; !exist {
		mpsPerPayloadSinceLastMeasurement[p] = 0
		mpsPerPayloadMeasured[p] = 0
		messageCountPerPayload[p] = 0
	}
	// else just update value
	mpsPerPayloadSinceLastMeasurement[p]++
	// increase cumulative metrics
	messageCountPerPayload[p]++
	atomic.AddUint64(&messageTotalCount, 1)
}

func measureMPSPerPayload() {
	messageLock.Lock()
	defer messageLock.Unlock()

	for payloadType, sinceLast := range mpsPerPayloadSinceLastMeasurement {
		// measure
		mpsPerPayloadMeasured[payloadType] = sinceLast
		// reset counter
		mpsPerPayloadSinceLastMeasurement[payloadType] = 0
	}
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.TipSelector.TipCount()))
}
