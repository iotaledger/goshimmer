package module

import (
	"sync"
)

type Interface interface {
	Lifecycle() *LifecycleEvents
}

type Module struct {
	lifecycleEvents *LifecycleEvents
	initOnce        sync.Once
}

func (m *Module) Lifecycle() *LifecycleEvents {
	m.initOnce.Do(func() {
		m.lifecycleEvents = NewLifecycleEvents()
	})

	return m.lifecycleEvents
}
