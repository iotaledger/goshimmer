package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	queueSizePerNode  *prometheus.GaugeVec
	manaAmountPerNode *prometheus.GaugeVec
	schedulerRate     prometheus.Gauge
	readyBlocksCount  prometheus.Gauge
	totalBlocksCount  prometheus.Gauge
	bufferSize        prometheus.Gauge
	maxBufferSize     prometheus.Gauge
	schedulerDeficit  prometheus.Gauge
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
		Help: "rate at which blocks are scheduled (in millisecond).",
	})

	readyBlocksCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_ready_blk_count",
		Help: "number of ready blocks in the scheduler buffer.",
	})

	totalBlocksCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_total_blk_count",
		Help: "number of  blocks in the scheduler buffer.",
	})

	bufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_total_size",
		Help: "number of bytes waiting to be scheduled.",
	})

	maxBufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_buffer_max_size",
		Help: "maximum number of bytes that can be stored in the buffer.",
	})

	schedulerDeficit = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_deficit",
		Help: "deficit value for local node.",
	})

	registry.MustRegister(queueSizePerNode)
	registry.MustRegister(manaAmountPerNode)
	registry.MustRegister(schedulerRate)
	registry.MustRegister(readyBlocksCount)
	registry.MustRegister(totalBlocksCount)
	registry.MustRegister(bufferSize)
	registry.MustRegister(maxBufferSize)
	registry.MustRegister(schedulerDeficit)

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
	readyBlocksCount.Set(float64(metrics.SchedulerReadyBlocksCount()))
	totalBlocksCount.Set(float64(metrics.SchedulerTotalBufferBlocksCount()))
	bufferSize.Set(float64(metrics.SchedulerBufferSize()))
	maxBufferSize.Set(float64(metrics.SchedulerMaxBufferSize()))
	schedulerDeficit.Set(metrics.SchedulerDeficit())
}
