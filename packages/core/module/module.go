package module

import (
	"sync"

	"github.com/iotaledger/hive.go/runtime/event"
)

type Module struct {
	constructed *event.Event
	initialized *event.Event
	stopped     *event.Event

	initOnce sync.Once
}

func (m *Module) TriggerConstructed() {
	m.initOnce.Do(m.init)

	m.constructed.Trigger()
}

func (m *Module) WasConstructed() bool {
	m.initOnce.Do(m.init)

	return m.constructed.WasTriggered()
}

func (m *Module) HookConstructed(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.constructed.Hook(callback, opts...)
}

func (m *Module) TriggerInitialized() {
	m.initOnce.Do(m.init)

	m.initialized.Trigger()
}

func (m *Module) WasInitialized() bool {
	m.initOnce.Do(m.init)

	return m.initialized.WasTriggered()
}

func (m *Module) HookInitialized(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.initialized.Hook(callback, opts...)
}

func (m *Module) TriggerStopped() {
	m.initOnce.Do(m.init)

	m.stopped.Trigger()
}

func (m *Module) WasStopped() bool {
	m.initOnce.Do(m.init)

	return m.stopped.WasTriggered()
}

func (m *Module) HookStopped(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.stopped.Hook(callback, opts...)
}

func (m *Module) init() {
	m.constructed = event.New(event.WithMaxTriggerCount(1))
	m.initialized = event.New(event.WithMaxTriggerCount(1))
	m.stopped = event.New(event.WithMaxTriggerCount(1))
}

var _ Interface = &Module{}
