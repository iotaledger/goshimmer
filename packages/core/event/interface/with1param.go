package _interface

type With1Param[A any] struct {
	*base
}

func New1[A any]() *With1Param[A] {
	return &With1Param[A]{
		base: newBase(),
	}
}

func (w *With1Param[A]) Trigger(a A) {
	if w.triggerCount.Add(1) >= w.maxTriggerCount && w.maxTriggerCount > 0 {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook) bool {
		if hook.triggerCount.Add(1) >= hook.maxTriggerCount && hook.maxTriggerCount > 0 {
			return true
		}

		if hook.workerPool == nil {
			hook.callback.(func(A))(a)

			return true
		}

		hook.workerPool.Submit(func() {
			hook.callback.(func(A))(a)
		})

		return true
	})
}
