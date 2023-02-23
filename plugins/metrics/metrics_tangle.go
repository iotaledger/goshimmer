package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/event"
)

const (
	tangleNamespace = "tangle"

	tipsCount                     = "tips_count"
	blockPerTypeCount             = "block_per_type_total"
	missingBlocksCount            = "missing_block_total"
	parentPerTypeCount            = "parent_per_type_total"
	blocksPerComponentCount       = "blocks_per_component_total"
	timeSinceReceivedPerComponent = "time_since_received_per_component_seconds"
	requestQueueSize              = "request_queue_size"
	blocksOrphanedCount           = "blocks_orphaned_total"
	acceptedBlocksCount           = "accepted_blocks_count"
)

var TangleMetrics = collector.NewCollection(tangleNamespace,
	collector.WithMetric(collector.NewMetric(tipsCount,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of tips in the tangle"),
		collector.WithCollectFunc(func() map[string]float64 {
			count := deps.Protocol.TipManager.TipCount()
			return collector.SingleValue(count)
		}),
	)),
	collector.WithMetric(collector.NewMetric(blockPerTypeCount,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of blocks per type in the tangle"),
		collector.WithLabels("type"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				blockType := collector.NewBlockType(block.Payload().Type()).String()
				deps.Collector.Increment(tangleNamespace, blockPerTypeCount, blockType)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingBlocksCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of blocks missing during the solidification in the tangle"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Hook(func(_ *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, missingBlocksCount)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(parentPerTypeCount,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Number of parents of the block per its type"),
		collector.WithLabels("type"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				blockType := collector.NewBlockType(block.Payload().Type()).String()
				block.ForEachParent(func(parent models.Parent) {
					deps.Collector.Increment(tangleNamespace, parentPerTypeCount, blockType)
				})
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(blocksPerComponentCount,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Number of blocks per component"),
		collector.WithLabels("component"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Network.BlockReceived.Hook(func(_ *network.BlockReceivedEvent) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Received.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.Engine.Filter.BlockAllowed.Hook(func(_ *models.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Allowed.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.BlockIssuer.Events.BlockIssued.Hook(func(_ *models.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Issued.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Attached.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Hook(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Solidified.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Scheduled.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Hook(func(_ *booker.BlockBookedEvent) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Booked.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Hook(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.SchedulerDropped.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockSkipped.Hook(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.SchedulerSkipped.String())
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(blocksOrphanedCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of orphaned blocks"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksOrphanedCount)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedBlocksCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of accepted blocks"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				deps.Collector.Increment(tangleNamespace, acceptedBlocksCount)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(timeSinceReceivedPerComponent,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Time since the block was received per component"),
		collector.WithLabels("component"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				blockType := collector.NewBlockType(block.Payload().Type()).String()
				timeSince := float64(time.Since(block.IssuingTime()).Milliseconds())
				deps.Collector.Update(tangleNamespace, timeSinceReceivedPerComponent, collector.MultiLabelsValues([]string{blockType}, timeSince))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(requestQueueSize,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of blocks in the request queue"),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(float64(deps.Protocol.Engine().BlockRequester.QueueSize()))
		}),
	)),
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
