package metrics

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
)

const (
	workerPoolNamespace = "workerpool"

	workers = "workers"
	tasks   = "tasks_pending"

	retainerLabel = "retainer"
)

var WorkerPoolMetrics = collector.NewCollection(workerPoolNamespace,
	collector.WithMetric(collector.NewMetric(workers,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of workers in the worker pool."),
		collector.WithLabels("type"),
		collector.WithCollectFunc(func() map[string]float64 {
			collected := make(map[string]float64)

			for _, p := range Plugin.Node.LoadedPlugins() {
				collected[p.Name] = float64(p.WorkerPool.WorkerCount())
			}

			if deps.Protocol != nil {
				for name, wp := range deps.Protocol.Workers.Pools() {
					collected[name] = float64(wp.WorkerCount())
				}
			}

			if deps.Retainer != nil {
				for name, wp := range deps.Retainer.Workers.Pools() {
					collected[name] = float64(wp.WorkerCount())
				}
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

			for _, p := range Plugin.Node.LoadedPlugins() {
				collected[p.Name] = float64(p.WorkerPool.PendingTasksCounter.Get())
			}

			if deps.Protocol != nil {
				for name, wp := range deps.Protocol.Workers.Pools() {
					collected[name] = float64(wp.PendingTasksCounter.Get())
				}
			}

			if deps.Retainer != nil {
				for name, wp := range deps.Retainer.Workers.Pools() {
					collected[name] = float64(wp.PendingTasksCounter.Get())
				}
			}

			return collected
		}),
	)),
)
