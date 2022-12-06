package traits

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

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
		linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

// initializable is the implementation of the Initializable trait.
type initializable struct {
	// linkable is the linkable Event that is used for the subscriptions.
	linkable *event.Linkable[bool]

	// optCallbacks is a list of optional callbacks that are triggered when the component is initialized.
	optCallbacks []func()

	// initializedTriggered is true if the initialized event was triggered.
	initializedTriggered bool

	// initializedTriggeredMutex is used to make the initializedTriggered flag thread-safe.
	initializedTriggeredMutex sync.RWMutex
}

// SubscribeInitialized registers a new callback that is triggered when the component was initialized.
func (i *initializable) SubscribeInitialized(callback func()) (unsubscribe func()) {
	closure := event.NewClosure(func(bool) {
		callback()
	})

	i.linkable.Attach(closure)

	return func() {
		i.linkable.Detach(closure)
	}
}

// TriggerInitialized triggers the initialized event.
func (i *initializable) TriggerInitialized() {
	i.initializedTriggeredMutex.Lock()
	defer i.initializedTriggeredMutex.Unlock()

	if i.initializedTriggered {
		return
	}

	i.initializedTriggered = true

	for _, optCallback := range i.optCallbacks {
		optCallback()
	}

	i.linkable.Trigger(true)
}

// WasInitialized returns true if the initialized event was triggered.
func (i *initializable) WasInitialized() (initialized bool) {
	i.initializedTriggeredMutex.RLock()
	defer i.initializedTriggeredMutex.RUnlock()

	return i.initializedTriggered
}
