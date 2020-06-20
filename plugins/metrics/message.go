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
	// init counter with zero value if we see the payload for the first time
	if _, exist := messageCountPerPayload[p]; !exist {
		messageCountPerPayload[p] = 0
	}
	// increase cumulative metrics
	messageCountPerPayload[p]++
	messageTotalCount.Inc()
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger((uint64)(messagelayer.TipSelector.TipCount()))
}
