package traits

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// Constructable is a trait that allows to subscribe to and trigger an event, whenever a component was constructed.
type Constructable interface {
	// SubscribeConstructed registers a new callback that is triggered when the component was constructed.
	SubscribeConstructed(callback func()) (unsubscribe func())

	// TriggerConstructed triggers the constructed event.
	TriggerConstructed()

	// WasConstructed returns true if the constructed event was triggered.
	WasConstructed() (wasConstructed bool)
}

// NewConstructable creates a new Constructable trait.
func NewConstructable(optCallbacks ...func()) (newConstructable Constructable) {
	return &constructable{
		linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

// constructable is the implementation of the Constructable trait.
type constructable struct {
	// linkable is the linkable Event that is used for the subscriptions.
	linkable *event.Linkable[bool]

	// optCallbacks is a list of optional callbacks that are triggered when the component is constructed.
	optCallbacks []func()

	// constructedTriggered is true if the constructed event was triggered.
	constructedTriggered bool

	// constructedTriggeredMutex is used to make the constructedTriggered flag thread-safe.
	constructedTriggeredMutex sync.RWMutex
}

// SubscribeConstructed registers a new callback that is triggered when the component was constructed.
func (c *constructable) SubscribeConstructed(callback func()) (unsubscribe func()) {
	closure := event.NewClosure(func(bool) {
		callback()
	})

	c.linkable.Attach(closure)

	return func() {
		c.linkable.Detach(closure)
	}
}

// TriggerConstructed triggers the constructed event.
func (c *constructable) TriggerConstructed() {
	c.constructedTriggeredMutex.Lock()
	defer c.constructedTriggeredMutex.Unlock()

	if c.constructedTriggered {
		return
	}

	c.constructedTriggered = true

	for _, optCallback := range c.optCallbacks {
		optCallback()
	}

	c.linkable.Trigger(true)
}

// WasConstructed returns true if the constructed event was triggered.
func (c *constructable) WasConstructed() (initialized bool) {
	c.constructedTriggeredMutex.RLock()
	defer c.constructedTriggeredMutex.RUnlock()

	return c.constructedTriggered
}
