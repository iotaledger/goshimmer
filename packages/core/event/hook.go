package event

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/workerpool"
)

type Hook[TriggerFuncType any] struct {
	callback        TriggerFuncType
	workerPool      *workerpool.UnboundedWorkerPool
	triggerCount    atomic.Uint64
	maxTriggerCount uint64
	unhook          func()
	unhookOnce      sync.Once
}

func (h *Hook[F]) Unhook() {
	h.unhookOnce.Do(h.unhook)
}

func (h *Hook[TriggerFuncType]) shouldTrigger() bool {
	if h.triggerCount.Add(1) > h.maxTriggerCount && h.maxTriggerCount > 0 {
		return false
	}

	return true
}

func (h *Hook[TriggerFuncType]) setMaxTriggerCount(maxTriggerCount uint64) {
	h.maxTriggerCount = maxTriggerCount
}

func (h *Hook[TriggerFuncType]) setWorkerPool(workerPool *workerpool.UnboundedWorkerPool) {
	h.workerPool = workerPool
}

var _ configurable = &Hook[any]{}
