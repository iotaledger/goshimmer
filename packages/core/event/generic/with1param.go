package generic

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
)

type With1Param[Param1 any] struct {
	*base[func(Param1)]
}

func NewWith1Param[Param1 any](opts ...Option) *With1Param[Param1] {
	return &With1Param[Param1]{
		base: newBase[func(Param1)](opts...),
	}
}

func (w *With1Param[Param1]) Trigger(param1 Param1) {
	if !w.shouldTrigger() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func(Param1)]) bool {
		if !hook.shouldTrigger() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.callback(param1)
		} else {
			workerPool.Submit(func() { hook.callback(param1) })
		}

		return true
	})
}

func (w *With1Param[A]) LinkTo(optTarget ...*With1Param[A]) {
	w.linkTo(w.Trigger, lo.First(optTarget))
}
