//go:build ignore

package event

//go:generate go run events_generate.go 9

// Event /*-paramCount*/ is an event that can be triggered to invoke a set of hooked callbacks.
type Event /*-paramCount*/ /*-Constraints*/ struct {
	*base[func( /*-Types-*/ )]
}

// New /*-paramCount*/ creates a new event with /*ParamCount*/ parameters.
func New /*-paramCount-*/ /*-Constraints-*/ (opts ...Option) *Event /*-paramCount*/ /*-constraints*/ {
	return &Event /*-paramCount-*/ /*-constraints*/ {
		base: newEvent[func( /*-Types-*/ )](opts...),
	}
}

// Trigger triggers the event and invokes the hooked callbacks.
func (e *Event /*-paramCount-*/ /*-constraints-*/) Trigger( /*-Params-*/ ) {
	if e.MaxTriggerCountReached() {
		return
	}

	e.hooks.ForEach(func(_ uint64, hook *Hook[func( /*-Types-*/ )]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := e.targetWorkerPool(hook); workerPool == nil {
			hook.trigger( /*-params-*/ )
		} else {
			workerPool.Submit(func() {
				hook.trigger( /*-params-*/ )
			})
		}

		return true
	})
}

// LinkTo links the event to the given target.
func (e *Event /*-paramCount-*/ /*-constraints-*/) LinkTo(target *Event /*-paramCount-*/ /*-constraints-*/) {
	e.linkTo(e.Trigger, target)
}
