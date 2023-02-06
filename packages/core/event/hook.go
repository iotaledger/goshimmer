package event

type Hook[FuncType any] struct {
	trigger FuncType
	unhook  func()

	triggerSettings
}

func newHook[TriggerFunc any](trigger TriggerFunc, unhook func(), opts ...Option) *Hook[TriggerFunc] {
	return &Hook[TriggerFunc]{
		trigger:         trigger,
		unhook:          unhook,
		triggerSettings: *newTriggerSettings(opts...),
	}
}

func (h *Hook[TriggerFunc]) Unhook() {
	h.unhook()
}
