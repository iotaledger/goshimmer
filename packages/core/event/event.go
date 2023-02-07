package event

//go:generate go run variadics_generate.go 9

// Event /*-paramCount*/ is an event with /*ParamCount*/ generic parameters.
type Event /*-paramCount*/ /*-Constraints*/ struct {
	*base[func( /*-Types-*/ )]
}

// New /*-paramCount*/ creates a new Event /*-paramCount*/ object.
func New /*-paramCount-*/ /*-Constraints-*/ (opts ...Option) *Event /*-paramCount*/ /*-constraints*/ {
	return &Event /*-paramCount-*/ /*-constraints*/ {
		base: newBase[func( /*-Types-*/ )](opts...),
	}
}

// Trigger invokes the hooked callbacks with the given parameters.
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

// LinkTo links the event to the given target event (nil unlinks).
func (e *Event /*-paramCount-*/ /*-constraints-*/) LinkTo(target *Event /*-paramCount-*/ /*-constraints-*/) {
	e.linkTo(target, e.Trigger)
}
