package event

import (
	"github.com/iotaledger/hive.go/core/generics/options"
)

type Hook[FuncType any] struct {
	trigger FuncType
	unhook  func()

	*triggerSettings
}

func newHook[TriggerFunc any](trigger TriggerFunc, unhook func(), opts ...options.Option[triggerSettings]) *Hook[TriggerFunc] {
	return &Hook[TriggerFunc]{
		trigger:         trigger,
		unhook:          unhook,
		triggerSettings: options.Apply(new(triggerSettings), opts),
	}
}

func (h *Hook[TriggerFunc]) Unhook() {
	h.unhook()
}
