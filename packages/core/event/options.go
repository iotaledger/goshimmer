package event

import (
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/workerpool"
)

type Option func(triggerable *triggerSettings)

func WithMaxTriggerCount(maxTriggerCount uint64) Option {
	return func(configurable *triggerSettings) {
		configurable.setMaxTriggerCount(maxTriggerCount)
	}
}

func WithWorkerPool(workerPool *workerpool.UnboundedWorkerPool) Option {
	return func(configurable *triggerSettings) {
		configurable.setWorkerPool(workerPool)
	}
}

// triggerSettings is a helper struct that can be embedded into an event to provide the trigger functionality.
type triggerSettings struct {
	workerPool      *workerpool.UnboundedWorkerPool
	triggerCount    atomic.Uint64
	maxTriggerCount uint64
}

func newTriggerSettings(opts ...Option) *triggerSettings {
	c := new(triggerSettings)

	for _, option := range opts {
		option(c)
	}

	return c
}

func (t *triggerSettings) WasTriggered() bool {
	return t.triggerCount.Load() > 0
}

func (t *triggerSettings) TriggerCount() uint64 {
	return t.triggerCount.Load()
}

func (t *triggerSettings) MaxTriggerCount() uint64 {
	return t.maxTriggerCount
}

func (t *triggerSettings) MaxTriggerCountReached() bool {
	return t.triggerCount.Add(1) > t.maxTriggerCount && t.maxTriggerCount != 0
}

func (t *triggerSettings) setMaxTriggerCount(maxTriggerCount uint64) {
	t.maxTriggerCount = maxTriggerCount
}

func (t *triggerSettings) setWorkerPool(workerPool *workerpool.UnboundedWorkerPool) {
	t.workerPool = workerPool
}
