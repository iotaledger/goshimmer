package prometheus

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pendingTasks *prometheus.GaugeVec
	workerCount  *prometheus.GaugeVec
)

func registerWorkerPoolMetrics() {
	workerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "workerpool",
		Name:      "workers",
		Help:      "Number of workers in the worker pool.",
	}, []string{"type"})

	pendingTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "workerpool",
		Name:      "tasks_pending",
		Help:      "Number of pending tasks in the worker pool..",
	}, []string{"type"})

	registry.MustRegister(pendingTasks)
	registry.MustRegister(workerCount)

	addCollect(collectWorkerPoolMetrics)
	if deps.Retainer != nil {
		addCollect(collectRetainerWorkerPoolMetric)
	}
}

func collectWorkerPoolMetrics() {
	workerCount.WithLabelValues("eventloop").Set(float64(event.Loop.WorkerCount()))
	pendingTasks.WithLabelValues("eventloop").Set(float64(event.Loop.PendingTasksCounter.Value()))

	if deps.Protocol != nil {
		for name, wp := range deps.Protocol.WorkerPools() {
			workerCount.WithLabelValues(name).Set(float64(wp.WorkerCount()))
			pendingTasks.WithLabelValues(name).Set(float64(wp.PendingTasksCounter.Value()))
		}
	}
}

func collectRetainerWorkerPoolMetric() {
	workerCount.WithLabelValues("retainer").Set(float64(deps.Retainer.WorkerPool().WorkerCount()))
	pendingTasks.WithLabelValues("retainer").Set(float64(deps.Retainer.WorkerPool().PendingTasksCounter.Value()))
}
