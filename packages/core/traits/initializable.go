package traits

// Initializable is a trait that allows to subscribe to and trigger an event, whenever a component was initialized.
type Initializable interface {
	// SubscribeInitialized registers a new callback that is triggered when the component was initialized.
	SubscribeInitialized(callback func()) (unsubscribe func())

	// TriggerInitialized triggers the initialized event.
	TriggerInitialized()

	// WasInitialized returns true if the initialized event was triggered.
	WasInitialized() (wasInitialized bool)
}

// NewInitializable creates a new Initializable trait.
func NewInitializable(optCallbacks ...func()) (newInitializable Initializable) {
	return &initializable{
		lifecycleEvent: newLifecycleEvent(optCallbacks...),
	}
}

func SubscribeInitialized(initializers map[Initializable]func()) (unsubscribe func()) {
	unsubscribeCallbacks := make([]func(), 0)
	for dependency, initCallback := range initializers {
		unsubscribeCallbacks = append(unsubscribeCallbacks, dependency.SubscribeInitialized(initCallback))
	}

	return func() {
		for _, unsubscribeCallback := range unsubscribeCallbacks {
			unsubscribeCallback()
		}
	}
}

// initializable is the implementation of the Initializable trait.
type initializable struct {
	lifecycleEvent *lifecycleEvent
}

// SubscribeInitialized registers a new callback that is triggered when the component was initialized.
func (i *initializable) SubscribeInitialized(callback func()) (unsubscribe func()) {
	return i.lifecycleEvent.Subscribe(callback)
}

// TriggerInitialized triggers the initialized event.
func (i *initializable) TriggerInitialized() {
	i.lifecycleEvent.Trigger()
}

// WasInitialized returns true if the initialized event was triggered.
func (i *initializable) WasInitialized() (initialized bool) {
	return i.lifecycleEvent.WasTriggered()
}
