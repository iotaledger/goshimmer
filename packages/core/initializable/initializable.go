package initializable

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

type Initializable struct {
	linkable *event.Linkable[bool]

	optCallbacks              []func()
	initializedTriggered      bool
	initializedTriggeredMutex sync.RWMutex
}

func New(optCallbacks ...func()) (initializable *Initializable) {
	return &Initializable{
		linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

func (c *Initializable) SubscribeInitialized(callback func()) (unsubscribe func()) {
	closure := event.NewClosure(func(bool) {
		callback()
	})

	c.linkable.Attach(closure)

	return func() {
		c.linkable.Detach(closure)
	}
}

func (c *Initializable) WasInitialized() (initialized bool) {
	c.initializedTriggeredMutex.RLock()
	defer c.initializedTriggeredMutex.RUnlock()

	return c.initializedTriggered
}

func (c *Initializable) TriggerInitialized() {
	c.initializedTriggeredMutex.Lock()
	defer c.initializedTriggeredMutex.Unlock()

	if c.initializedTriggered {
		return
	}

	c.initializedTriggered = true

	for _, optCallback := range c.optCallbacks {
		optCallback()
	}

	c.linkable.Trigger(true)
}
