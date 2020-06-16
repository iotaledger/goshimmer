package metrics

import (
	"go.uber.org/atomic"
)

// ReceivedMessagesPerSecond retrieves the current messages per second number.
func ReceivedMessagesPerSecond() uint64 {
	return measuredReceivedMPS.Load()
}

// counter for the received MPS
var mpsReceivedSinceLastMeasurement atomic.Uint64

// measured value of the received MPS
var measuredReceivedMPS atomic.Uint64

// increases the received MPS counter
func increaseReceivedMPSCounter() {
	mpsReceivedSinceLastMeasurement.Add(1)
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
