package event

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
)

type ParamEvent2[Param1, Param2 any] struct {
	*event[func(Param1, Param2)]
}

func New2[Param1, Param2 any](opts ...Option) *ParamEvent2[Param1, Param2] {
	return &ParamEvent2[Param1, Param2]{
		event: newEvent[func(Param1, Param2)](opts...),
	}
}

func (w *ParamEvent2[Param1, Param2]) Trigger(param1 Param1, param2 Param2) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(Param1, Param2)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(param1, param2)
		} else {
			workerPool.Submit(func() { hook.trigger(param1, param2) })
		}

		return true
	})
}

func (w *ParamEvent2[Param1, Param2]) LinkTo(optTarget ...*ParamEvent2[Param1, Param2]) {
	w.linkTo(w.Trigger, lo.First(optTarget))
}
