package module

import (
	"sync"

	"github.com/iotaledger/hive.go/runtime/event"
)

// Module is a trait that exposes a lifecycle related API, that can be used to create a modular architecture where
// different modules can listen and wait for each other to reach certain states.
type Module struct {
	// constructed is triggered when the module was constructed.
	constructed *event.Event

	// initialized is triggered when the module was initialized.
	initialized *event.Event

	// stopped is triggered when the module was stopped.
	stopped *event.Event

	// initOnce is used to ensure that the init function is only called once.
	initOnce sync.Once
}

// TriggerConstructed triggers the constructed event.
func (m *Module) TriggerConstructed() {
	m.initOnce.Do(m.init)

	m.constructed.Trigger()
}

// WasConstructed returns true if the constructed event was triggered.
func (m *Module) WasConstructed() bool {
	m.initOnce.Do(m.init)

	return m.constructed.WasTriggered()
}

// HookConstructed registers a callback for the constructed event.
func (m *Module) HookConstructed(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.constructed.Hook(callback, opts...)
}

// TriggerInitialized triggers the initialized event.
func (m *Module) TriggerInitialized() {
	m.initOnce.Do(m.init)

	m.initialized.Trigger()
}

// WasInitialized returns true if the initialized event was triggered.
func (m *Module) WasInitialized() bool {
	m.initOnce.Do(m.init)

	return m.initialized.WasTriggered()
}

// HookInitialized registers a callback for the initialized event.
func (m *Module) HookInitialized(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.initialized.Hook(callback, opts...)
}

// TriggerStopped triggers the stopped event.
func (m *Module) TriggerStopped() {
	m.initOnce.Do(m.init)

	m.stopped.Trigger()
}

// WasStopped returns true if the stopped event was triggered.
func (m *Module) WasStopped() bool {
	m.initOnce.Do(m.init)

	return m.stopped.WasTriggered()
}

// HookStopped registers a callback for the stopped event.
func (m *Module) HookStopped(callback func(), opts ...event.Option) *event.Hook[func()] {
	m.initOnce.Do(m.init)

	return m.stopped.Hook(callback, opts...)
}

// init initializes the module.
func (m *Module) init() {
	m.constructed = event.New(event.WithMaxTriggerCount(1))
	m.initialized = event.New(event.WithMaxTriggerCount(1))
	m.stopped = event.New(event.WithMaxTriggerCount(1))
}

// code contract (make sure the type implements all required methods).
var _ Interface = &Module{}
