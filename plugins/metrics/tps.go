package metrics

import (
	"sync/atomic"
)

// ReceivedMessagesPerSecond retrieves the current messages per second number.
func ReceivedMessagesPerSecond() uint64 {
	return atomic.LoadUint64(&measuredReceivedMPS)
}

// counter for the received MPS
var mpsReceivedSinceLastMeasurement uint64

// measured value of the received MPS
var measuredReceivedMPS uint64

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
