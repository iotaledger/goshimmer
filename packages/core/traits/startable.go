package traits

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

type Startable interface {
	SubscribeStartup(callback func()) (unsubscribe func())
	TriggerStartup()
	WasStarted() (wasStarted bool)
}

func NewStartable(optCallbacks ...func()) (newStartable Startable) {
	return &startable{
		linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

type startable struct {
	linkable *event.Linkable[bool]

	optCallbacks              []func()
	initializedTriggered      bool
	initializedTriggeredMutex sync.RWMutex
}

func (c *startable) SubscribeStartup(callback func()) (unsubscribe func()) {
	closure := event.NewClosure(func(bool) {
		callback()
	})

	c.linkable.Attach(closure)

	return func() {
		c.linkable.Detach(closure)
	}
}

func (c *startable) WasStarted() (initialized bool) {
	c.initializedTriggeredMutex.RLock()
	defer c.initializedTriggeredMutex.RUnlock()

	return c.initializedTriggered
}

func (c *startable) TriggerStartup() {
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
