package prometheus

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/prometheus/client_golang/prometheus"
)

var workerCount prometheus.Gauge
var pendingTasks prometheus.Gauge

func registerWorkerPoolMetrics() {
	workerCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "workerpool",
			Subsystem: "eventloop",
			Name:      "workers",
			Help:      "Number of workers in the event loop worker pool.",
		},
	)
	pendingTasks = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "workerpool",
			Subsystem: "eventloop",
			Name:      "tasks_pending",
			Help:      "Number of pending tasks in the event loop worker pool.",
		},
	)

	registry.MustRegister(pendingTasks)
	registry.MustRegister(workerCount)

	addCollect(collectWorkerPoolMetrics)
}

func collectWorkerPoolMetrics() {
	workerCount.Set(float64(event.Loop.WorkerCount()))
	pendingTasks.Set(float64(event.Loop.PendingTasksCounter.Value()))
}
