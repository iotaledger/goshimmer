package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	tangleSubsystem = "tangle"

	tipsCount         = "tips_count"
	blockPerTypeCount = "block_per_type_count"
)

var TangleMetrics = collector.NewCollection(tangleSubsystem,
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
				blockType := collector.NewBlockType(block.Payload().Type())
				labVal := map[string]float64{
					blockType.String(): 1,
				}
				deps.Collector.Update(tangleSubsystem, blockPerTypeCount, labVal)
			}))
		}),
	)),
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var ConflictMetrics = collector.NewCollection("conflict")
