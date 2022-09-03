package requester

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// Events represents events happening on a block requester.
type Events struct {
	// BlockRequested is an event that is triggered when the requester wants to request the given Block from its
	// neighbors.
	BlockRequested *event.Event[models.BlockID]

	// RequestStarted is an event that is triggered when a new request is started.
	RequestStarted *event.Event[models.BlockID]

	// RequestStopped is an event that is triggered when a request is stopped.
	RequestStopped *event.Event[models.BlockID]

	// RequestFailed is an event that is triggered when a request is stopped after too many attempts.
	RequestFailed *event.Event[models.BlockID]
}

// newEvents creates a new Events instance.
func newEvents() (events *Events) {
	return &Events{
		BlockRequested: event.New[models.BlockID](),
		RequestStarted: event.New[models.BlockID](),
		RequestStopped: event.New[models.BlockID](),
		RequestFailed:  event.New[models.BlockID](),
	}
}
