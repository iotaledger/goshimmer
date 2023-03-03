package module

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

type LifecycleEvents struct {
	Constructed *event.Event
	Initialized *event.Event
	Stopped     *event.Event

	event.Group[LifecycleEvents, *LifecycleEvents]
}

var NewLifecycleEvents = event.CreateGroupConstructor(func() (self *LifecycleEvents) {
	return &LifecycleEvents{
		Constructed: event.New(event.WithMaxTriggerCount(1)),
		Initialized: event.New(event.WithMaxTriggerCount(1)),
		Stopped:     event.New(event.WithMaxTriggerCount(1)),
	}
})
