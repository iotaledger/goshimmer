package module

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

// Interface defines the interface of a module.
type Interface interface {
	// TriggerConstructed triggers the constructed event.
	TriggerConstructed()

	// WasConstructed returns true if the constructed event was triggered.
	WasConstructed() bool

	// HookConstructed registers a callback for the constructed event.
	HookConstructed(func(), ...event.Option) *event.Hook[func()]

	// TriggerInitialized triggers the initialized event.
	TriggerInitialized()

	// WasInitialized returns true if the initialized event was triggered.
	WasInitialized() bool

	// HookInitialized registers a callback for the initialized event.
	HookInitialized(func(), ...event.Option) *event.Hook[func()]

	// TriggerStopped triggers the stopped event.
	TriggerStopped()

	// WasStopped returns true if the stopped event was triggered.
	WasStopped() bool

	// HookStopped registers a callback for the stopped event.
	HookStopped(func(), ...event.Option) *event.Hook[func()]
}
