package metrics

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
)

const (
	schedulerNamespace = "scheduler"

	queueSizePerNode      = "queue_size_per_node_bytes"
	manaAmountPerNode     = "mana_per_node"
	bufferReadyBlockCount = "buffer_ready_block_total"
	bufferTotalSize       = "buffer_size_bytes_total"
	bufferMaxSize         = "buffer_max_size"
	deficit               = "deficit"
	rate                  = "rate"
)

var SchedulerMetrics = collector.NewCollection(schedulerNamespace,
	collector.WithMetric(collector.NewMetric(queueSizePerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current size of each node's queue (in bytes)."),
		collector.WithCollectFunc(func() map[string]float64 {
			res := make(map[string]float64)
			for issuer, s := range deps.Protocol.CongestionControl.Scheduler().IssuerQueueSizes() {
				res[issuer.String()] = float64(s)
			}
			return res
		}),
	)),
	collector.WithMetric(collector.NewMetric(manaAmountPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of aMana of each node in the queue (in bytes)."),
		collector.WithCollectFunc(func() map[string]float64 {
			res := make(map[string]float64)
			for issuer, val := range deps.Protocol.CongestionControl.Scheduler().GetAccessManaMap() {
				res[issuer.String()] = float64(val)
			}
			return res
		}),
	)),
	collector.WithMetric(collector.NewMetric(bufferMaxSize,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Maximum number of bytes that can be stored in the buffer."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.CongestionControl.Scheduler().MaxBufferSize())
		}),
	)),
	collector.WithMetric(collector.NewMetric(bufferReadyBlockCount,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of ready blocks in the scheduler buffer."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.CongestionControl.Scheduler().ReadyBlocksCount())
		}),
	)),
	collector.WithMetric(collector.NewMetric(bufferTotalSize,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current size of the scheduler buffer (in bytes)."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.CongestionControl.Scheduler().BufferSize())
		}),
	)),
	collector.WithMetric(collector.NewMetric(deficit,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current deficit of the scheduler."),
		collector.WithCollectFunc(func() map[string]float64 {
			deficit, _ := deps.Protocol.CongestionControl.Scheduler().Deficit(deps.Local.ID()).Float64()
			return collector.SingleValue(deficit)
		}),
	)),
	collector.WithMetric(collector.NewMetric(rate,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current rate of the scheduler."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.CongestionControl.Scheduler().Rate())
		}),
	)),
)
