package event

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
)

type ParamEvent1[Param1 any] struct {
	*event[func(Param1)]
}

func New1[Param1 any](opts ...Option) *ParamEvent1[Param1] {
	return &ParamEvent1[Param1]{
		event: newEvent[func(Param1)](opts...),
	}
}

func (w *ParamEvent1[Param1]) Trigger(param1 Param1) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(Param1)]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger(param1)
		} else {
			workerPool.Submit(func() { hook.trigger(param1) })
		}

		return true
	})
}

func (w *ParamEvent1[A]) LinkTo(optTarget ...*ParamEvent1[A]) {
	w.linkTo(w.Trigger, lo.First(optTarget))
}
