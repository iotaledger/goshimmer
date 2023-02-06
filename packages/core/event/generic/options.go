package generic

import (
	"github.com/iotaledger/hive.go/core/workerpool"
)

type Option func(configurable configurable)

func WithMaxTriggerCount(maxTriggerCount uint64) Option {
	return func(configurable configurable) {
		configurable.setMaxTriggerCount(maxTriggerCount)
	}
}

func WithWorkerPool(workerPool *workerpool.UnboundedWorkerPool) Option {
	return func(configurable configurable) {
		configurable.setWorkerPool(workerPool)
	}
}
