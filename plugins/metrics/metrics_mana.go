package metrics

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/hive.go/lo"
)

const (
	manaNamespace = "mana"

	manaPerNode        = "access_per_node"
	weightPerNode      = "consensus_per_node"
	nodeManaPercentile = "node_percentile"
	neighborsMana      = "neighbors"
)

var ManaMetrics = collector.NewCollection(manaNamespace,
	collector.WithMetric(collector.NewMetric(manaPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("nodeID"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of aMana of each node in the network."),
		collector.WithCollectFunc(func() map[string]float64 {
			access := deps.Protocol.Engine().ThroughputQuota.BalanceByIDs()
			res := make(map[string]float64)
			for id, val := range access {
				res[id.String()] = float64(val)
			}
			return res
		}),
	)),
	collector.WithMetric(collector.NewMetric(weightPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("nodeID"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of cMana of each node in the network."),
		collector.WithCollectFunc(func() map[string]float64 {
			consensus := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())
			res := make(map[string]float64)
			for id, val := range consensus {
				res[id.String()] = float64(val)
			}
			return res
		}),
	)),
	collector.WithMetric(collector.NewMetric(nodeManaPercentile,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("type"),
		collector.WithHelp("Current percentile of the local node in terms of aMana."),
		collector.WithCollectFunc(func() map[string]float64 {
			access := deps.Protocol.Engine().ThroughputQuota.BalanceByIDs()
			accPerc := manamodels.Percentile(deps.Local.ID(), access)
			consensus := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())
			conPerc := manamodels.Percentile(deps.Local.ID(), consensus)

			return collector.MultiLabelsValues([]string{"access", "consensus"}, accPerc, conPerc)
		}),
	)),
	collector.WithMetric(collector.NewMetric(neighborsMana,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("type"),
		collector.WithHelp(""),
		collector.WithCollectFunc(func() map[string]float64 {
			neighbors := deps.P2Pmgr.AllNeighbors()
			access := deps.Protocol.Engine().ThroughputQuota.BalanceByIDs()
			consensus := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())

			accAvg := manamodels.NeighborsAverageMana(access, neighbors)
			conAvg := manamodels.NeighborsAverageMana(consensus, neighbors)
			return collector.MultiLabelsValues([]string{"access", "consensus"}, accAvg, conAvg)
		}),
	)),
)
