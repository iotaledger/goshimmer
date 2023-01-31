package traits

// Constructable is a trait that allows to subscribe to and trigger an event, whenever a component was constructed.
type Constructable interface {
	// SubscribeConstructed registers callbacks that are triggered when the component was constructed.
	SubscribeConstructed(callbacks ...func()) (unsubscribe func())

	// TriggerConstructed triggers the constructed event.
	TriggerConstructed()

	// WasConstructed returns true if the constructed event was triggered.
	WasConstructed() (wasConstructed bool)
}

// NewConstructable creates a new Constructable trait.
func NewConstructable(optCallbacks ...func()) (newConstructable Constructable) {
	return &constructable{
		lifecycleEvent: newLifecycleEvent(optCallbacks...),
	}
}

// constructable is the implementation of the Constructable trait.
type constructable struct {
	lifecycleEvent *lifecycleEvent
}

// SubscribeConstructed registers callbacks that are triggered when the component was constructed.
func (c *constructable) SubscribeConstructed(callbacks ...func()) (unsubscribe func()) {
	return c.lifecycleEvent.Subscribe(callbacks...)
}

// TriggerConstructed triggers the constructed event.
func (c *constructable) TriggerConstructed() {
	c.lifecycleEvent.Trigger()
}

// WasConstructed returns true if the constructed event was triggered.
func (c *constructable) WasConstructed() (initialized bool) {
	return c.lifecycleEvent.WasTriggered()
}
