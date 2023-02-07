package event

import (
	"github.com/iotaledger/hive.go/core/generics/options"
)

// Hook is a container that holds a hook function and its settings.
type Hook[FuncType any] struct {
	trigger FuncType
	unhook  func()

	*triggerSettings
}

// newHook creates a new Hook.
func newHook[TriggerFunc any](trigger TriggerFunc, unhook func(), opts ...options.Option[triggerSettings]) *Hook[TriggerFunc] {
	return &Hook[TriggerFunc]{
		trigger:         trigger,
		unhook:          unhook,
		triggerSettings: options.Apply(new(triggerSettings), opts),
	}
}

// Unhook removes the hook from the event.
func (h *Hook[TriggerFunc]) Unhook() {
	h.unhook()
}
