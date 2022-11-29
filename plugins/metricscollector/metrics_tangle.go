package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
)

var TangleMetrics = collector.NewCollection("tangle",
	collector.WithMetric(collector.NewMetric("tips_count",
		collector.WithType(collector.Gauge),
		collector.WithHelp("Number of tips in the tangle"),
		collector.WithCollectFunc(func() float64 {
			count := deps.Protocol.TipManager.TipCount()
			return float64(count)
		})),
	))

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var ConflictMetrics = collector.NewCollection("conflict")
