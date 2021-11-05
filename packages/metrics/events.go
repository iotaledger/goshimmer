package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

// CollectionEvents defines the events fot the metrics package.
type CollectionEvents struct {
	// AnalysisOutboundBytes defines the local analysis outbound network traffic in bytes.
	AnalysisOutboundBytes *events.Event
	// CPUUsage defines the local CPU usage.
	CPUUsage *events.Event
	// MemUsage defines the local GoShimmer memory usage.
	MemUsage *events.Event
	// TangleTimeSynced defines the local sync status event based on tangle time.
	TangleTimeSynced *events.Event
	// ValueTips defines the local value tips count event.
	ValueTips *events.Event
	// MessageTips defines the local message tips count event.
	MessageTips *events.Event
}

func uint64Caller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}

func float64Caller(handler interface{}, params ...interface{}) {
	handler.(func(float64))(params[0].(float64))
}

func boolCaller(handler interface{}, params ...interface{}) {
	handler.(func(bool))(params[0].(bool))
}
