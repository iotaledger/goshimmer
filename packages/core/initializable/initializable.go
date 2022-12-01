package initializable

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
)

type Initializable struct {
	*event.Linkable[bool]

	optCallbacks              []func()
	initializedTriggered      bool
	initializedTriggeredMutex sync.RWMutex
}

func NewInitializable(optCallbacks ...func()) (initializable *Initializable) {
	return &Initializable{
		Linkable:     event.NewLinkable[bool](),
		optCallbacks: optCallbacks,
	}
}

func (c *Initializable) WasTriggered() (initialized bool) {
	c.initializedTriggeredMutex.RLock()
	defer c.initializedTriggeredMutex.RUnlock()

	return c.initializedTriggered
}

func (c *Initializable) Trigger() {
	c.initializedTriggeredMutex.Lock()
	defer c.initializedTriggeredMutex.Unlock()

	if c.initializedTriggered {
		return
	}

	c.initializedTriggered = true

	for _, optCallback := range c.optCallbacks {
		optCallback()
	}

	c.Linkable.Trigger(true)
}
