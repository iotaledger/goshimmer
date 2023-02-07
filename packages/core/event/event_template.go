//go:build ignore

package event

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/workerpool"
)

type event[TriggerFunc any] struct {
	hooks        *orderedmap.OrderedMap[uint64, *Hook[TriggerFunc]]
	hooksCounter atomic.Uint64
	link         *Hook[TriggerFunc]
	linkMutex    sync.Mutex

	*triggerSettings
}

func newEvent[TriggerFunc any](opts ...Option) *event[TriggerFunc] {
	b := &event[TriggerFunc]{
		hooks:           orderedmap.New[uint64, *Hook[TriggerFunc]](),
		triggerSettings: options.Apply(new(triggerSettings), opts),
	}

	for _, option := range opts {
		option(b.triggerSettings)
	}

	return b
}

func (e *event[TriggerFunc]) Hook(triggerFunc TriggerFunc, opts ...Option) (hook *Hook[TriggerFunc]) {
	hookID := e.hooksCounter.Add(1)
	hook = newHook(triggerFunc, func() { e.hooks.Delete(hookID) }, opts...)

	e.hooks.Set(hookID, hook)

	return hook
}

func (e *event[TriggerFunc]) linkTo(triggerFunc TriggerFunc, target hookable[TriggerFunc]) {
	e.linkMutex.Lock()
	defer e.linkMutex.Unlock()

	if e.link != nil {
		e.link.Unhook()
	}

	if target == nil {
		e.link = nil
	} else {
		e.link = target.Hook(triggerFunc)
	}
}

func (e *event[TriggerFunc]) targetWorkerPool(hook *Hook[TriggerFunc]) (workerPool *workerpool.UnboundedWorkerPool) {
	if hook.workerPool != nil {
		return hook.workerPool
	}

	return e.workerPool
}

type hookable[TriggerFunc any] interface {
	Hook(callback TriggerFunc, opts ...Option) (hook *Hook[TriggerFunc])
}

//go:generate go run event_generate.go 9

// Event /*-paramCount*/ is an event with /*ParamCount*/ generic parameters.
type Event /*-paramCount*/ /*-Constraints*/ struct {
	*event[func( /*-Types-*/ )]
}

// New /*-paramCount*/ creates a new Event /*-paramCount*/ object.
func New /*-paramCount-*/ /*-Constraints-*/ (opts ...Option) *Event /*-paramCount*/ /*-constraints*/ {
	return &Event /*-paramCount-*/ /*-constraints*/ {
		event: newEvent[func( /*-Types-*/ )](opts...),
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

// LinkTo links the event to the given target (nil unlinks the event).
func (e *Event /*-paramCount-*/ /*-constraints-*/) LinkTo(target *Event /*-paramCount-*/ /*-constraints-*/) {
	e.linkTo(e.Trigger, target)
}
