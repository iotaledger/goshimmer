package traits

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
func NewStoppable(optCallbacks ...func()) (newStoppable Stoppable) {
	return &stoppable{
		lifecycleEvent: newLifecycleEvent(optCallbacks...),
	}
}

// stoppable is the implementation of the Stoppable trait.
type stoppable struct {
	lifecycleEvent *lifecycleEvent
}

// SubscribeStopped registers a new callback that is triggered when the component was stopped.
func (s *stoppable) SubscribeStopped(callback func()) (unsubscribe func()) {
	return s.lifecycleEvent.Subscribe(callback)
}

// TriggerStopped triggers the stopped event.
func (s *stoppable) TriggerStopped() {
	s.lifecycleEvent.Trigger()
}

// WasStopped returns true if the stopped event was triggered.
func (s *stoppable) WasStopped() (stopped bool) {
	return s.lifecycleEvent.WasTriggered()
}
