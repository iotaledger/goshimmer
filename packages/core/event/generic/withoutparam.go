package generic

type WithOutParam struct {
	*base[func()]
}

func NewWithOutParam() *WithOutParam {
	return &WithOutParam{
		base: newBase[func()](),
	}
}

func (w *WithOutParam) Trigger() {
	if w.triggerCount.Add(1) < w.maxTriggerCount || w.maxTriggerCount == 0 {
		w.hooks.ForEach(func(_ uint64, hook *Hook[func()]) bool {
			if hook.triggerCount.Add(1) >= hook.maxTriggerCount && hook.maxTriggerCount > 0 {
				return true
			}

			if hook.workerPool == nil {
				hook.callback()

				return true
			}

			hook.workerPool.Submit(hook.callback)

			return true
		})
	}
}
