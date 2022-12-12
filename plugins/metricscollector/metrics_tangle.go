package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	tangleNamespace = "tangle"

	tipsCount               = "tips_count"
	blockPerTypeCount       = "block_per_type_count"
	missingBlocksCount      = "missing_block_count"
	parentPerTypeCount      = "parent_per_type_count"
	blocksPerComponentCount = "blocks_per_component_count"
	// todo finish dbStatsResult in /prometheus/block.go that were commented out due to ???
	timeSinceReceivedPerComponent = "time_since_received_per_component"
	// todo finish measureRequestQueueSize when requester done: requestQueueSize
	requestQueueSize    = "request_queue_size"
	blocksOrphanedCount = "blocks_orphaned_count"
	acceptedBlocksCount = "accepted_blocks_count"
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
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
				blockType := collector.NewBlockType(block.Payload().Type()).String()
				deps.Collector.Increment(tangleNamespace, blockPerTypeCount, blockType)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingBlocksCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of blocks missing during the solidification in the tangle"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(_ *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, missingBlocksCount)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(parentPerTypeCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of parents of the block per its type"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
				blockType := collector.NewBlockType(block.Payload().Type()).String()
				block.ForEachParent(func(parent models.Parent) {
					deps.Collector.Increment(tangleNamespace, parentPerTypeCount, blockType)
				})
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(blocksPerComponentCount,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Number of blocks per component"),
		collector.WithLabels("component"),
		collector.WithInitFunc(func() {
			deps.Protocol.Network().Events.BlockReceived.Attach(event.NewClosure(func(_ *network.BlockReceivedEvent) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Received.String())
			}))
			deps.Protocol.Events.Engine.Filter.BlockAllowed.Attach(event.NewClosure(func(_ *models.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Allowed.String())
			}))
			deps.BlockIssuer.RateSetter.Events.BlockIssued.Attach(event.NewClosure(func(_ *models.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Issued.String())
			}))
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Attached.String())
			}))
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Solidified.String())
			}))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Scheduled.String())
			}))
			deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.Booked.String())
			}))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.SchedulerDropped.String())
			}))
			deps.Protocol.Events.CongestionControl.Scheduler.BlockSkipped.Attach(event.NewClosure(func(block *scheduler.Block) {
				deps.Collector.Increment(tangleNamespace, blocksPerComponentCount, collector.SchedulerSkipped.String())
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(blocksOrphanedCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of orphaned blocks"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
				deps.Collector.Increment(tangleNamespace, blocksOrphanedCount)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedBlocksCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of accepted blocks"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
				deps.Collector.Increment(tangleNamespace, acceptedBlocksCount)
			}))
		}),
	)),
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
