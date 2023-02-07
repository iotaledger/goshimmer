// Code generated automatically DO NOT EDIT.
package event

// Event is an event that can be triggered to invoke a set of hooked callbacks.
type Event struct {
	*base[func()]
}

// New creates a new event with no parameters.
func New(opts ...Option) *Event {
	return &Event {
		base: newEvent[func()](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event) Trigger() {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func()]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger()
		} else {
			workerPool.Submit(func() {
				hook.trigger()
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event) LinkTo(target *Event) {
	w.linkTo(w.Trigger, target)
}

// Event1 is an event that can be triggered to invoke a set of hooked callbacks.
type Event1[T1 any] struct {
	*base[func(T1)]
}

// New1 creates a new event with 1 parameters.
func New1[T1 any](opts ...Option) *Event1[T1] {
	return &Event1[T1] {
		base: newEvent[func(T1)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event1[T1]) Trigger(arg1 T1) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event1[T1]) LinkTo(target *Event1[T1]) {
	w.linkTo(w.Trigger, target)
}

// Event2 is an event that can be triggered to invoke a set of hooked callbacks.
type Event2[T1, T2 any] struct {
	*base[func(T1, T2)]
}

// New2 creates a new event with 2 parameters.
func New2[T1, T2 any](opts ...Option) *Event2[T1, T2] {
	return &Event2[T1, T2] {
		base: newEvent[func(T1, T2)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event2[T1, T2]) Trigger(arg1 T1, arg2 T2) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event2[T1, T2]) LinkTo(target *Event2[T1, T2]) {
	w.linkTo(w.Trigger, target)
}

// Event3 is an event that can be triggered to invoke a set of hooked callbacks.
type Event3[T1, T2, T3 any] struct {
	*base[func(T1, T2, T3)]
}

// New3 creates a new event with 3 parameters.
func New3[T1, T2, T3 any](opts ...Option) *Event3[T1, T2, T3] {
	return &Event3[T1, T2, T3] {
		base: newEvent[func(T1, T2, T3)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event3[T1, T2, T3]) Trigger(arg1 T1, arg2 T2, arg3 T3) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event3[T1, T2, T3]) LinkTo(target *Event3[T1, T2, T3]) {
	w.linkTo(w.Trigger, target)
}

// Event4 is an event that can be triggered to invoke a set of hooked callbacks.
type Event4[T1, T2, T3, T4 any] struct {
	*base[func(T1, T2, T3, T4)]
}

// New4 creates a new event with 4 parameters.
func New4[T1, T2, T3, T4 any](opts ...Option) *Event4[T1, T2, T3, T4] {
	return &Event4[T1, T2, T3, T4] {
		base: newEvent[func(T1, T2, T3, T4)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event4[T1, T2, T3, T4]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event4[T1, T2, T3, T4]) LinkTo(target *Event4[T1, T2, T3, T4]) {
	w.linkTo(w.Trigger, target)
}

// Event5 is an event that can be triggered to invoke a set of hooked callbacks.
type Event5[T1, T2, T3, T4, T5 any] struct {
	*base[func(T1, T2, T3, T4, T5)]
}

// New5 creates a new event with 5 parameters.
func New5[T1, T2, T3, T4, T5 any](opts ...Option) *Event5[T1, T2, T3, T4, T5] {
	return &Event5[T1, T2, T3, T4, T5] {
		base: newEvent[func(T1, T2, T3, T4, T5)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event5[T1, T2, T3, T4, T5]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4, T5)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4, arg5)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4, arg5)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event5[T1, T2, T3, T4, T5]) LinkTo(target *Event5[T1, T2, T3, T4, T5]) {
	w.linkTo(w.Trigger, target)
}

// Event6 is an event that can be triggered to invoke a set of hooked callbacks.
type Event6[T1, T2, T3, T4, T5, T6 any] struct {
	*base[func(T1, T2, T3, T4, T5, T6)]
}

// New6 creates a new event with 6 parameters.
func New6[T1, T2, T3, T4, T5, T6 any](opts ...Option) *Event6[T1, T2, T3, T4, T5, T6] {
	return &Event6[T1, T2, T3, T4, T5, T6] {
		base: newEvent[func(T1, T2, T3, T4, T5, T6)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event6[T1, T2, T3, T4, T5, T6]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5, arg6 T6) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4, T5, T6)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event6[T1, T2, T3, T4, T5, T6]) LinkTo(target *Event6[T1, T2, T3, T4, T5, T6]) {
	w.linkTo(w.Trigger, target)
}

// Event7 is an event that can be triggered to invoke a set of hooked callbacks.
type Event7[T1, T2, T3, T4, T5, T6, T7 any] struct {
	*base[func(T1, T2, T3, T4, T5, T6, T7)]
}

// New7 creates a new event with 7 parameters.
func New7[T1, T2, T3, T4, T5, T6, T7 any](opts ...Option) *Event7[T1, T2, T3, T4, T5, T6, T7] {
	return &Event7[T1, T2, T3, T4, T5, T6, T7] {
		base: newEvent[func(T1, T2, T3, T4, T5, T6, T7)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event7[T1, T2, T3, T4, T5, T6, T7]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5, arg6 T6, arg7 T7) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4, T5, T6, T7)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event7[T1, T2, T3, T4, T5, T6, T7]) LinkTo(target *Event7[T1, T2, T3, T4, T5, T6, T7]) {
	w.linkTo(w.Trigger, target)
}

// Event8 is an event that can be triggered to invoke a set of hooked callbacks.
type Event8[T1, T2, T3, T4, T5, T6, T7, T8 any] struct {
	*base[func(T1, T2, T3, T4, T5, T6, T7, T8)]
}

// New8 creates a new event with 8 parameters.
func New8[T1, T2, T3, T4, T5, T6, T7, T8 any](opts ...Option) *Event8[T1, T2, T3, T4, T5, T6, T7, T8] {
	return &Event8[T1, T2, T3, T4, T5, T6, T7, T8] {
		base: newEvent[func(T1, T2, T3, T4, T5, T6, T7, T8)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event8[T1, T2, T3, T4, T5, T6, T7, T8]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5, arg6 T6, arg7 T7, arg8 T8) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4, T5, T6, T7, T8)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event8[T1, T2, T3, T4, T5, T6, T7, T8]) LinkTo(target *Event8[T1, T2, T3, T4, T5, T6, T7, T8]) {
	w.linkTo(w.Trigger, target)
}

// Event9 is an event that can be triggered to invoke a set of hooked callbacks.
type Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9 any] struct {
	*base[func(T1, T2, T3, T4, T5, T6, T7, T8, T9)]
}

// New9 creates a new event with 9 parameters.
func New9[T1, T2, T3, T4, T5, T6, T7, T8, T9 any](opts ...Option) *Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
	return &Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
		base: newEvent[func(T1, T2, T3, T4, T5, T6, T7, T8, T9)](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (w *Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Trigger(arg1 T1, arg2 T2, arg3 T3, arg4 T4, arg5 T5, arg6 T6, arg7 T7, arg8 T8, arg9 T9) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(T1, T2, T3, T4, T5, T6, T7, T8, T9)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9)
		} else {
			workerPool.Submit(func() {
				hook.trigger(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9)
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (w *Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) LinkTo(target *Event9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) {
	w.linkTo(w.Trigger, target)
}
