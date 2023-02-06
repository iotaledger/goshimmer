package event

import (
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/workerpool"
)

// WithMaxTriggerCount sets the maximum number of times an event (or hook) shall be triggered.
func WithMaxTriggerCount(maxTriggerCount uint64) Option {
	return func(triggerSettings *triggerSettings) {
		triggerSettings.maxTriggerCount = maxTriggerCount
	}
}

// WithWorkerPool sets the worker pool that shall be used to execute the triggered function.
func WithWorkerPool(workerPool *workerpool.UnboundedWorkerPool) Option {
	return func(triggerSettings *triggerSettings) {
		triggerSettings.workerPool = workerPool
	}
}

// triggerSettings is a struct that contains trigger related settings and logic.
type triggerSettings struct {
	workerPool      *workerpool.UnboundedWorkerPool
	triggerCount    atomic.Uint64
	maxTriggerCount uint64
}

// WasTriggered returns true if Trigger was called at least once.
func (t *triggerSettings) WasTriggered() bool {
	return t.triggerCount.Load() > 0
}

// TriggerCount returns the number of times Trigger was called.
func (t *triggerSettings) TriggerCount() uint64 {
	return t.triggerCount.Load()
}

func (t *triggerSettings) MaxTriggerCount() uint64 {
	return t.maxTriggerCount
}

func (t *triggerSettings) MaxTriggerCountReached() bool {
	return t.triggerCount.Add(1) > t.maxTriggerCount && t.maxTriggerCount != 0
}

func (t *triggerSettings) WorkerPool() *workerpool.UnboundedWorkerPool {
	return t.workerPool
}

type Option = options.Option[triggerSettings]
