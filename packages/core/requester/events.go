package requester

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

// Events represents events happening on a block requester.
type Events struct {
	// RequestIssued is an event that is triggered when the requester wants to request the given Block from its
	// neighbors.
	RequestIssued *event.Event[tangle.BlockID]

	// RequestStarted is an event that is triggered when a new request is started.
	RequestStarted *event.Event[tangle.BlockID]

	// RequestStopped is an event that is triggered when a request is stopped.
	RequestStopped *event.Event[tangle.BlockID]

	// RequestFailed is an event that is triggered when a request is stopped after too many attempts.
	RequestFailed *event.Event[tangle.BlockID]
}

func newEvents() (new *Events) {
	return &Events{
		RequestIssued:  event.New[tangle.BlockID](),
		RequestStarted: event.New[tangle.BlockID](),
		RequestStopped: event.New[tangle.BlockID](),
		RequestFailed:  event.New[tangle.BlockID](),
	}
}
