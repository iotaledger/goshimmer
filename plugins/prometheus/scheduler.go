package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	queueSizePerNode   *prometheus.GaugeVec
	manaAmountPerNode  *prometheus.GaugeVec
	schedulerRate      prometheus.Gauge
	readyMessagesCount prometheus.Gauge
	totalMessagesCount prometheus.Gauge
	bufferSize         prometheus.Gauge
	maxBufferSize      prometheus.Gauge
)

func registerSchedulerMetrics() {
	queueSizePerNode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "scheduler_buffer_size_per_node",
			Help: "current size of each node's queue (in bytes).",
		}, []string{
			"node_id",
		})

	manaAmountPerNode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "scheduler_amana_per_node",
			Help: "current amount of aMana of each node in the queue (in bytes).",
		}, []string{
			"node_id",
		})

	schedulerRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_rate",
		Help: "rate at which messages are scheduled (in millisecond).",
	})

	readyMessagesCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_ready_msg_count",
		Help: "number of ready messages in the scheduler buffer.",
	})

	totalMessagesCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_total_msg_count",
		Help: "number of  messages in the scheduler buffer.",
	})

	bufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_total_size",
		Help: "number of bytes waiting to be scheduled.",
	})

	maxBufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_max_size",
		Help: "maximum number of bytes that can be stored in the buffer.",
	})

	registry.MustRegister(queueSizePerNode)
	registry.MustRegister(manaAmountPerNode)
	registry.MustRegister(schedulerRate)
	registry.MustRegister(readyMessagesCount)
	registry.MustRegister(totalMessagesCount)
	registry.MustRegister(bufferSize)
	registry.MustRegister(maxBufferSize)

	addCollect(collectSchedulerMetrics)
}

func collectSchedulerMetrics() {
	queueSizePerNode.Reset()
	nodeQueueSizeMap := metrics.SchedulerNodeQueueSizes()
	for currentNodeID, queueSize := range nodeQueueSizeMap {
		queueSizePerNode.WithLabelValues(currentNodeID).Set(float64(queueSize))
	}
	manaAmountPerNode.Reset()
	nodeAManaMap := metrics.SchedulerNodeAManaAmount()
	for currentNodeID, aMana := range nodeAManaMap {
		manaAmountPerNode.WithLabelValues(currentNodeID).Set(aMana)
	}
	schedulerRate.Set(float64(metrics.SchedulerRate()))
	readyMessagesCount.Set(float64(metrics.SchedulerReadyMessagesCount()))
	totalMessagesCount.Set(float64(metrics.SchedulerTotalBufferMessagesCount()))
	bufferSize.Set(float64(metrics.SchedulerBufferSize()))
	maxBufferSize.Set(float64(metrics.SchedulerMaxBufferSize()))
}
