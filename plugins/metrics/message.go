package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/syncutils"
)

// MPS retrieves the current messages per second number.
func MPS() uint64 {
	return atomic.LoadUint64(&measuredReceivedMPS)
}

// counter for the received MPS
var mpsReceivedSinceLastMeasurement uint64

// measured value of the received MPS
var measuredReceivedMPS uint64

// current number of message tips
var messageTips uint64

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

// MPS figures for different payload types
var mpsPerPayloadSinceLastMeasurement = make(map[payload.Type]uint64)
var mpsPerPayloadMeasured = make(map[payload.Type]uint64)
var mpsPerPayloadMu syncutils.RWMutex

func increasePerPayloadMPSCounter(p payload.Type) {
	mpsPerPayloadMu.Lock()
	defer mpsPerPayloadMu.Unlock()
	// init counter with zero value if we see the payload for the first time
	if _, exist := mpsPerPayloadSinceLastMeasurement[p]; !exist {
		mpsPerPayloadSinceLastMeasurement[p] = 0
		mpsPerPayloadMeasured[p] = 0
	}
	// else just update value
	mpsPerPayloadSinceLastMeasurement[p]++
}

func measureMPSPerPayload() {
	mpsPerPayloadMu.Lock()
	defer mpsPerPayloadMu.Unlock()

	for payloadType, sinceLast := range mpsPerPayloadSinceLastMeasurement {
		// measure
		mpsPerPayloadMeasured[payloadType] = sinceLast
		// reset counter
		mpsPerPayloadSinceLastMeasurement[payloadType] = 0
	}
}

// MPSPerPayload returns a map of message payload types and their corresponding MPS values.
func MPSPerPayload() map[payload.Type]uint64 {
	mpsPerPayloadMu.RLock()
	defer mpsPerPayloadMu.RUnlock()

	// copy the original map
	target := make(map[payload.Type]uint64)
	for key, element := range mpsPerPayloadMeasured {
		target[key] = element
	}

	return target
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.TipSelector.TipCount()))
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return atomic.LoadUint64(&messageTips)
}
