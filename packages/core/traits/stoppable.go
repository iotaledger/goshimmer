package traits

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// Stoppable is a trait that allows to subscribe to and trigger an event, whenever a component was stopped.
type Stoppable interface {
	// SubscribeStopped registers a new callback that is triggered when the component was stopped.
	SubscribeStopped(callback func()) (unsubscribe func())

	// TriggerStopped triggers the stopped event.
	TriggerStopped()

	// WasStopped returns true if the stopped event was triggered.
	WasStopped() (wasStopped bool)
}

// NewStoppable creates a new Stoppable trait.
func NewStoppable(optCallbacks ...func()) (newStopppable Stoppable) {
	return &stoppable{
		linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

// stoppable is the implementation of the Stoppable trait.
type stoppable struct {
	// linkable is the linkable Event that is used for the subscriptions.
	linkable *event.Linkable[bool]

	// optCallbacks is a list of optional callbacks that are triggered when the component is stopped.
	optCallbacks []func()

	// stoppedTriggered is true if the stopped event was triggered.
	stoppedTriggered bool

	// stoppedTriggeredMutex is used to make the stoppedTriggered flag thread-safe.
	stoppedTriggeredMutex sync.RWMutex
}

// SubscribeStopped registers a new callback that is triggered when the component was stopped.
func (s *stoppable) SubscribeStopped(callback func()) (unsubscribe func()) {
	closure := event.NewClosure(func(bool) {
		callback()
	})

	s.linkable.Hook(closure)

	return func() {
		s.linkable.Detach(closure)
	}
}

// TriggerStopped triggers the stopped event.
func (s *stoppable) TriggerStopped() {
	s.stoppedTriggeredMutex.Lock()
	defer s.stoppedTriggeredMutex.Unlock()

	if s.stoppedTriggered {
		return
	}

	s.stoppedTriggered = true

	for _, optCallback := range s.optCallbacks {
		optCallback()
	}

	s.linkable.Trigger(true)
}

// WasStopped returns true if the stopped event was triggered.
func (s *stoppable) WasStopped() (stopped bool) {
	s.stoppedTriggeredMutex.RLock()
	defer s.stoppedTriggeredMutex.RUnlock()

	return s.stoppedTriggered
}
