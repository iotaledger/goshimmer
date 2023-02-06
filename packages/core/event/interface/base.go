package _interface

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"go.uber.org/atomic"
)

type base struct {
	hooksCounter      uint64
	hooksCounterMutex sync.RWMutex

	hooks           *orderedmap.OrderedMap[uint64, *Hook]
	triggerCount    *atomic.Uint64
	maxTriggerCount uint64
}

func newBase() *base {
	return &base{
		hooks:        orderedmap.New[uint64, *Hook](),
		triggerCount: atomic.NewUint64(0),
	}
}

func (e *base) Hook(callback any, opts ...options.Option[Hook]) (hook *Hook) {
	e.hooksCounterMutex.Lock()
	hookID := e.hooksCounter
	e.hooksCounter++
	e.hooksCounterMutex.Unlock()

	hook = options.Apply(&Hook{
		id:       hookID,
		callback: callback,
		unhook: func() {
			e.hooks.Delete(hookID)
		},
	}, opts)

	e.hooks.Set(hook.id, hook)

	return hook
}

func (e *base) WasTriggered() bool {
	return e.triggerCount.Load() > 0
}

func (e *base) TriggerCount() uint64 {
	return e.triggerCount.Load()
}

func (e *base) trigger(hookCaller func(hook *Hook)) {
	if e.triggerCount.Inc() >= e.maxTriggerCount && e.maxTriggerCount > 0 {
		return
	}

	e.hooks.ForEach(func(_ uint64, hook *Hook) bool {
		if hook.triggerCount.Add(1) >= hook.maxTriggerCount && hook.maxTriggerCount > 0 {
			return true
		}

		if hook.workerPool == nil {
			hookCaller(hook)

			return true
		}

		hook.workerPool.Submit(func() {
			hookCaller(hook)
		})

		return true
	})
}
