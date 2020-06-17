package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/metrics"
)

// TPS retrieves the current transactions (value payloads) per second number.
func TPS() uint64 {
	return atomic.LoadUint64(&measuredReceivedTPS)
}

// counter for the received TPS (transactions per second)
var tpsReceivedSinceLastMeasurement uint64

// measured value of the received MPS
var measuredReceivedTPS uint64

// current number of value tips
var valueTips uint64

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

func measureValueTips() {
	metrics.Events().ValueTips.Trigger((uint64)(valuetransfers.TipManager().Size()))
}

// ValueTips returns the actual number of tips in the value tangle.
func ValueTips() uint64 {
	return atomic.LoadUint64(&valueTips)
}
