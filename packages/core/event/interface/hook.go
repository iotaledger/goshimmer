package _interface

import (
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/workerpool"
)

type Hook struct {
	id              uint64
	callback        any
	workerPool      *workerpool.UnboundedWorkerPool
	triggerCount    atomic.Uint64
	maxTriggerCount uint64
	unhook          func()
}

func (h *Hook) Unhook() {
	h.unhook()
}
