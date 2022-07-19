package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	blockTips                               prometheus.Gauge
	solidificationRequests                  prometheus.Gauge
	blockPerTypeCount                       *prometheus.GaugeVec
	initialBlockPerComponentCount           *prometheus.GaugeVec
	blockPerComponentCount                  *prometheus.GaugeVec
	initialSinceReceivedTotalTime           *prometheus.GaugeVec
	sinceReceivedTotalTime                  *prometheus.GaugeVec
	sinceIssuedTotalTime                    *prometheus.GaugeVec
	initialSchedulerTotalTime               prometheus.Gauge
	schedulerTotalTime                      prometheus.Gauge
	parentsCount                            *prometheus.GaugeVec
	initialMissingBlocksCountDB             prometheus.Gauge
	blockMissingCountDB                     prometheus.Gauge
	blockRequestCount                       prometheus.Gauge
	confirmedConflictCount                  prometheus.Gauge
	conflictConfirmationTotalTime           prometheus.Gauge
	totalConflictCountDB                    prometheus.Gauge
	finalizedConflictCountDB                prometheus.Gauge
	finalizedBlockCount                     *prometheus.GaugeVec
	blockFinalizationTotalTimeSinceReceived *prometheus.GaugeVec
	blockFinalizationTotalTimeSinceIssued   *prometheus.GaugeVec
)

func registerTangleMetrics() {
	blockTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_tips_count",
		Help: "Current number of tips in block tangle",
	})

	solidificationRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_solidification_missing_block_count",
		Help: "Total number of blocks requested by Solidifier.",
	})

	blockPerTypeCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_blocks_per_type_count",
			Help: "number of blocks per payload type seen since the start of the node",
		}, []string{
			"block_type",
		})

	parentsCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_blocks_parent_count_per_type",
			Help: "number of parents of all blocks",
		}, []string{
			"type",
		})
	initialBlockPerComponentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_initial_blocks_per_component_count",
			Help: "number of blocks per component seen since at the start of the node",
		}, []string{
			"component",
		})

	blockPerComponentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_blocks_per_component_count",
			Help: "number of blocks per component seen since the start of the node",
		}, []string{
			"component",
		})

	initialSinceReceivedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_initial_block_total_time_since_received",
			Help: "total time it took for a block to be processed by each component since it has been received at the start of the node",
		}, []string{
			"component",
		})

	sinceReceivedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_block_total_time_since_received",
			Help: "total time it took for a block to be processed by each component since it has been received, since the start of the node",
		}, []string{
			"component",
		})

	sinceIssuedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_block_total_time_since_issued",
			Help: "total time it took for a block to be processed by each component since it has been issued since the start of the node",
		}, []string{
			"component",
		})

	initialSchedulerTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_initial_total_scheduling_time",
		Help: "total time the scheduled blocks spend in the scheduling queue at the start of the node",
	})

	schedulerTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_total_scheduling_time",
		Help: "total time the scheduled blocks spend in the scheduling queue since the node start",
	})

	blockMissingCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_missing_initial_count_db",
		Help: "number of missing blocks in the node's database at the start of the node",
	})

	initialMissingBlocksCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_missing_count_db",
		Help: "number of missing blocks in the node's database since the start of the node",
	})

	blockRequestCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_block_request_queue_size",
		Help: "current number requested blocks by the block tangle",
	})

	blockFinalizationTotalTimeSinceReceived = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_block_finalization_time_since_received",
			Help: "total number of milliseconds taken for blocks to finalize since block received",
		}, []string{
			"blockType",
		})
	blockFinalizationTotalTimeSinceIssued = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_block_finalization_time_since_issued",
			Help: "total number of milliseconds taken for blocks to finalize since block issued",
		}, []string{
			"blockType",
		})

	finalizedBlockCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_block_finalized_count",
			Help: "current number of finalized blocks per type",
		}, []string{
			"blockType",
		})

	conflictConfirmationTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_conflict_confirmation_time",
		Help: "total number of milliseconds taken for conflict to finalize",
	})

	totalConflictCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_conflict_total_count_db",
		Help: "total number conflicts stored in database",
	})

	finalizedConflictCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_conflict_finalized_count_db",
		Help: "number of finalized conflicts stored in database",
	})
	confirmedConflictCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_conflict_confirmed_count",
		Help: "current number of confirmed conflicts",
	})

	registry.MustRegister(blockTips)
	registry.MustRegister(solidificationRequests)
	registry.MustRegister(blockPerTypeCount)
	registry.MustRegister(parentsCount)
	registry.MustRegister(initialBlockPerComponentCount)
	registry.MustRegister(blockPerComponentCount)
	registry.MustRegister(sinceIssuedTotalTime)
	registry.MustRegister(initialSinceReceivedTotalTime)
	registry.MustRegister(sinceReceivedTotalTime)
	registry.MustRegister(initialSchedulerTotalTime)
	registry.MustRegister(schedulerTotalTime)
	registry.MustRegister(initialMissingBlocksCountDB)
	registry.MustRegister(blockMissingCountDB)
	registry.MustRegister(blockRequestCount)
	registry.MustRegister(blockFinalizationTotalTimeSinceReceived)
	registry.MustRegister(blockFinalizationTotalTimeSinceIssued)
	registry.MustRegister(finalizedBlockCount)
	registry.MustRegister(conflictConfirmationTotalTime)
	registry.MustRegister(confirmedConflictCount)
	registry.MustRegister(totalConflictCountDB)
	registry.MustRegister(finalizedConflictCountDB)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	blockTips.Set(float64(metrics.BlockTips()))
	solidificationRequests.Set(float64(metrics.SolidificationRequests()))
	blkCountPerPayload := metrics.BlockCountSinceStartPerPayload()
	for payloadType, count := range blkCountPerPayload {
		blockPerTypeCount.WithLabelValues(payloadType.String()).Set(float64(count))
	}

	initialBlkCountPerComponent := metrics.InitialBlockCountPerComponentGrafana()
	for component, count := range initialBlkCountPerComponent {
		initialBlockPerComponentCount.WithLabelValues(component.String()).Set(float64(count))
	}

	blkCountPerComponent := metrics.BlockCountSinceStartPerComponentGrafana()
	for component, count := range blkCountPerComponent {
		blockPerComponentCount.WithLabelValues(component.String()).Set(float64(count))
	}

	initialSumTimeSinceReceived := metrics.InitialSumTimeSinceReceived()
	for component, count := range initialSumTimeSinceReceived {
		initialSinceReceivedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}

	sumTimeSinceReceived := metrics.SumTimeSinceReceived()
	for component, count := range sumTimeSinceReceived {
		sinceReceivedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}
	sumTimeSinceIssued := metrics.SumTimeSinceIssued()
	for component, count := range sumTimeSinceIssued {
		sinceIssuedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}

	initialSchedulerTotalTime.Set(float64(metrics.InitialSchedulerTime()))
	schedulerTotalTime.Set(float64(metrics.SchedulerTime()))
	initialMissingBlocksCountDB.Set(float64(metrics.InitialBlockMissingCountDB()))
	blockMissingCountDB.Set(float64(metrics.BlockMissingCountDB()))
	blockRequestCount.Set(float64(metrics.BlockRequestQueueSize()))
	confirmedConflictCount.Set(float64(metrics.ConfirmedConflictCount()))
	conflictConfirmationTotalTime.Set(float64(metrics.ConflictConfirmationTotalTime()))
	totalConflictCountDB.Set(float64(metrics.TotalConflictCountDB()))
	finalizedConflictCountDB.Set(float64(metrics.FinalizedConflictCountDB()))

	finalizedBlockCountPerType := metrics.FinalizedBlockCountPerType()
	for blockType, count := range finalizedBlockCountPerType {
		finalizedBlockCount.WithLabelValues(blockType.String()).Set(float64(count))
	}
	blockFinalizationTotalTimeSinceReceivePerType := metrics.BlockFinalizationTotalTimeSinceReceivedPerType()
	for blockType, count := range blockFinalizationTotalTimeSinceReceivePerType {
		blockFinalizationTotalTimeSinceReceived.WithLabelValues(blockType.String()).Set(float64(count))
	}
	blockFinalizationTotalTimeSinceIssuederType := metrics.BlockFinalizationTotalTimeSinceIssuedPerType()
	for blockType, count := range blockFinalizationTotalTimeSinceIssuederType {
		blockFinalizationTotalTimeSinceIssued.WithLabelValues(blockType.String()).Set(float64(count))
	}

	parentsCountPerType := metrics.ParentCountPerType()
	for parentType, count := range parentsCountPerType {
		parentsCount.WithLabelValues(parentType.String()).Set(float64(count))
	}
}
