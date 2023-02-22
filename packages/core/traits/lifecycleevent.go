package traits

import (
	"sync"

	"github.com/iotaledger/hive.go/runtime/event"
)

// newLifecycleEvent creates a new lifecycle event trait.
func newLifecycleEvent(optCallbacks ...func()) (newLifecycleEvent *lifecycleEvent) {
	return &lifecycleEvent{
		event:     event.New1[bool](),
		callbacks: optCallbacks,
	}
}

// lifecycleEvent is the implementation of the lifecycle event trait.
type lifecycleEvent struct {
	// event is the linkable Event that is used for the subscriptions.
	event *event.Event1[bool]

	// optCallbacks is a list of optional callbacks that are triggered when the component is stopped.
	callbacks []func()

	// triggered is true if the stopped event was triggered.
	triggered bool

	// mutex is used to make the triggered flag thread-safe.
	mutex sync.RWMutex
}

// Subscribe registers callbacks that are triggered when the event was triggered.
func (s *lifecycleEvent) Subscribe(callbacks ...func()) (unsubscribe func()) {
	if len(callbacks) == 0 {
		return func() {}
	}

	return s.event.Hook(func(bool) {
		for _, callback := range callbacks {
			callback()
		}
	}).Unhook
}

// Trigger triggers the event.
func (s *lifecycleEvent) Trigger() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.triggered {
		return
	}

	s.triggered = true

	for _, optCallback := range s.callbacks {
		optCallback()
	}

	s.event.Trigger(true)
}

// WasTriggered returns true if the event was triggered.
func (s *lifecycleEvent) WasTriggered() (stopped bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.triggered
}
