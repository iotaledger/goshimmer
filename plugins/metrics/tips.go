package metrics

import (
	"sync/atomic"
)

// public api method to proactively retrieve the received TPS value
func GetTips() uint64 {
	return atomic.LoadUint64(&totalTips)
}

// counter for remain tips
var totalTips uint64

// increases the received TPS counter
func increaseTipsCounter() {
	atomic.AddUint64(&totalTips, 1)
}

// decreases the received TPS counter
func decreaseTipsCounter() {
	atomic.AddUint64(&totalTips, ^uint64(0))
}
