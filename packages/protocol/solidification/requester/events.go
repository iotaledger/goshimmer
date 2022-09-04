package requester

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// Events represents events happening on a block requester.
type Events struct {
	// BlockRequested is an event that is triggered when the requester wants to request the given Block from its
	// neighbors.
	BlockRequested *event.Linkable[models.BlockID, Events, *Events]

	// RequestStarted is an event that is triggered when a new request is started.
	RequestStarted *event.Linkable[models.BlockID, Events, *Events]

	// RequestStopped is an event that is triggered when a request is stopped.
	RequestStopped *event.Linkable[models.BlockID, Events, *Events]

	// RequestFailed is an event that is triggered when a request is stopped after too many attempts.
	RequestFailed *event.Linkable[models.BlockID, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.NewLinkableEvents(func(self *Events) (linker func(*Events)) {
	self.BlockRequested = event.NewLinkable[models.BlockID, Events]()
	self.RequestStarted = event.NewLinkable[models.BlockID, Events]()
	self.RequestStopped = event.NewLinkable[models.BlockID, Events]()
	self.RequestFailed = event.NewLinkable[models.BlockID, Events]()

	return func(newTarget *Events) {
		self.BlockRequested.LinkTo(newTarget.BlockRequested)
		self.RequestStarted.LinkTo(newTarget.RequestStarted)
		self.RequestStopped.LinkTo(newTarget.RequestStopped)
		self.RequestFailed.LinkTo(newTarget.RequestFailed)
	}
})
