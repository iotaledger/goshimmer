package metrics

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"go.uber.org/atomic"
)

var (
	// counter of value transaction.
	valueTransactionCounter atomic.Uint64
	// current number of value tips.
	valueTips atomic.Uint64
)

func measureValueTips() {
	metrics.Events().ValueTips.Trigger((uint64)(valuetransfers.TipManager().Size()))
}

// ValueTransactionCounter returns the number of value transactions seen.
func ValueTransactionCounter() uint64 {
	return valueTransactionCounter.Load()
}

// ValueTips returns the actual number of tips in the value tangle.
func ValueTips() uint64 {
	return valueTips.Load()
}
