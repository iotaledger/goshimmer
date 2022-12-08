package metrics

import "github.com/iotaledger/hive.go/core/generics/event"

// CollectionEvents defines the events fot the metrics package.
type CollectionEvents struct {
	// AnalysisOutboundBytes defines the local analysis outbound network traffic in bytes.
	AnalysisOutboundBytes *event.Event[*AnalysisOutboundBytesEvent]
}

func newCollectionEvents() (new *CollectionEvents) {
	return &CollectionEvents{
		AnalysisOutboundBytes: event.New[*AnalysisOutboundBytesEvent](),
	}
}

type AnalysisOutboundBytesEvent struct {
	AmountBytes uint64
}
