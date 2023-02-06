package generic

type With2Params[Param1, Param2 any] struct {
	*base[func(Param1, Param2)]
}

func NewWith2Params[Param1, Param2 any]() *With2Params[Param1, Param2] {
	return &With2Params[Param1, Param2]{
		base: newBase[func(Param1, Param2)](),
	}
}

func (w *With2Params[Param1, Param2]) Trigger(param1 Param1, param2 Param2) {
	if w.triggerCount.Add(1) < w.maxTriggerCount || w.maxTriggerCount == 0 {
		w.hooks.ForEach(func(_ uint64, hook *Hook[func(Param1, Param2)]) bool {
			if hook.shouldTrigger() {
				if workerPool := w.targetWorkerPool(hook); workerPool == nil {
					hook.callback(param1, param2)
				} else {
					workerPool.Submit(func() { hook.callback(param1, param2) })
				}
			}

			return true
		})
	}
}
