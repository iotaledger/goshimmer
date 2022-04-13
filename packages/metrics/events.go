package metrics

import "github.com/iotaledger/hive.go/generics/event"

// CollectionEvents defines the events fot the metrics package.
type CollectionEvents struct {
	// AnalysisOutboundBytes defines the local analysis outbound network traffic in bytes.
	AnalysisOutboundBytes *event.Event[*AnalysisOutboundBytesEvent]
	// CPUUsage defines the local CPU usage.
	CPUUsage *event.Event[*CPUUsageEvent]
	// MemUsage defines the local GoShimmer memory usage.
	MemUsage *event.Event[*MemUsageEvent]
	// TangleTimeSynced defines the local sync status event based on tangle time.
	TangleTimeSynced *event.Event[*TangleTimeSyncedEvent]
	// ValueTips defines the local value tips count event.
	ValueTips *event.Event[*ValueTipsEvent]
	// MessageTips defines the local message tips count event.
	MessageTips *event.Event[*MessageTipsEvent]
}

func newCollectionEvents() (new *CollectionEvents) {
	return &CollectionEvents{
		AnalysisOutboundBytes: event.New[*AnalysisOutboundBytesEvent](),
		CPUUsage:              event.New[*CPUUsageEvent](),
		MemUsage:              event.New[*MemUsageEvent](),
		TangleTimeSynced:      event.New[*TangleTimeSyncedEvent](),
		ValueTips:             event.New[*ValueTipsEvent](),
		MessageTips:           event.New[*MessageTipsEvent](),
	}
}

type AnalysisOutboundBytesEvent struct {
}

type CPUUsageEvent struct {
}

type MemUsageEvent struct {
}

type TangleTimeSyncedEvent struct {
}

type ValueTipsEvent struct {
}

type MessageTipsEvent struct {
}
