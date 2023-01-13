package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	workerPoolNamespace = "workerpool"

	workers = "eventloop_workers"
	tasks   = "eventloop_tasks_pending"
)

var WorkerPoolMetrics = collector.NewCollection(workerPoolNamespace,
	collector.WithMetric(collector.NewMetric(workers,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of workers in the event loop worker pool."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(float64(event.Loop.WorkerCount()))
		}),
	)),
	collector.WithMetric(collector.NewMetric(tasks,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of pending tasks in the event loop worker pool."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(float64(event.Loop.PendingTasksCounter.Value()))
		}),
	)),
)
