package metrics

import (
	"sync/atomic"
)

// public api method to proactively retrieve the received TPS value
func GetReceivedTPS() uint64 {
	return atomic.LoadUint64(&measuredReceivedTPS)
}

// counter for the received TPS
var tpsReceivedSinceLastMeasurement uint64

// measured value of the received TPS
var measuredReceivedTPS uint64

// increases the received TPS counter
func increaseReceivedTPSCounter() {
	atomic.AddUint64(&tpsReceivedSinceLastMeasurement, 1)
}

// measures the received TPS value
func measureReceivedTPS() {
	// sample the current counter value into a measured TPS value
	sampledTPS := atomic.LoadUint64(&tpsReceivedSinceLastMeasurement)

	// store the measured value
	atomic.StoreUint64(&measuredReceivedTPS, sampledTPS)

	// reset the counter
	atomic.StoreUint64(&tpsReceivedSinceLastMeasurement, 0)

	// trigger events for outside listeners
	Events.ReceivedTPSUpdated.Trigger(sampledTPS)
}
