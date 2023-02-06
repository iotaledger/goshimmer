package event

import (
	"github.com/iotaledger/hive.go/core/workerpool"
)

type Hookable[TriggerFuncType any] interface {
	Hook(callback TriggerFuncType, opts ...Option) (hook *Hook[TriggerFuncType])
}

type configurable interface {
	setMaxTriggerCount(maxTriggerCount uint64)
	setWorkerPool(workerPool *workerpool.UnboundedWorkerPool)
}
