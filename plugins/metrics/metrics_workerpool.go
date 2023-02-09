package metrics

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/app/collector"
)

const (
	workerPoolNamespace = "workerpool"

	workers = "workers"
	tasks   = "tasks_pending"

	eventLoopLabel = "event_loop"
	retainerLabel  = "retainer"
)

var WorkerPoolMetrics = collector.NewCollection(workerPoolNamespace,
	collector.WithMetric(collector.NewMetric(workers,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of workers in the worker pool."),
		collector.WithLabels("type"),
		collector.WithCollectFunc(func() map[string]float64 {
			collected := make(map[string]float64)

			collected[eventLoopLabel] = float64(event.Loop.WorkerCount())

			if deps.Protocol != nil {
				for name, wp := range deps.Protocol.Workers.Pools() {
					collected[name] = float64(wp.WorkerCount())
				}
			}

			if deps.Retainer != nil {
				collected[retainerLabel] = float64(deps.Retainer.WorkerPool().WorkerCount())
			}

			return collected
		}),
	)),
	collector.WithMetric(collector.NewMetric(tasks,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of pending tasks in the worker pool."),
		collector.WithLabels("type"),
		collector.WithCollectFunc(func() map[string]float64 {
			collected := make(map[string]float64)

			collected[eventLoopLabel] = float64(event.Loop.PendingTasksCounter.Get())

			if deps.Protocol != nil {
				for name, wp := range deps.Protocol.Workers.Pools() {
					collected[name] = float64(wp.PendingTasksCounter.Get())
				}
			}

			if deps.Retainer != nil {
				collected[retainerLabel] = float64(deps.Retainer.WorkerPool().PendingTasksCounter.Get())
			}

			return collected
		}),
	)),
)
