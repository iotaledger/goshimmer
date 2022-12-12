package metricscollector

import "github.com/iotaledger/goshimmer/packages/app/collector"

const (
	manaNamespace = "mana"

	accessManaPerNode    = "access_mana_per_node"
	consensusManaPerNode = "consensus_mana_per_node"

	// todo pecentile, avgs
)

var ManaMetrics = collector.NewCollection(manaNamespace,
	collector.WithMetric(collector.NewMetric(accessManaPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of aMana of each node in the network."),
		collector.WithCollectFunc(func() map[string]float64 {
			// todo
			return nil
		}),
	)),
	collector.WithMetric(collector.NewMetric(consensusManaPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of cMana of each node in the network."),
		collector.WithCollectFunc(func() map[string]float64 {
			// todo
			return nil
		}),
	)),
)
