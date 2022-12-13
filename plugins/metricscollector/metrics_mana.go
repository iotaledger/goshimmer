package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/manatracker/manamodels"
	"github.com/iotaledger/hive.go/core/generics/lo"
)

const (
	manaNamespace = "mana"

	manaPerNode        = "access_per_node"
	weightPerNode      = "consensus_per_node"
	nodeManaPercentile = "node_mana_percentile"
	neighborsMana      = "neighbors_mana"
)

var ManaMetrics = collector.NewCollection(manaNamespace,
	collector.WithMetric(collector.NewMetric(manaPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
		collector.WithResetBeforeCollecting(true),
		collector.WithHelp("Current amount of aMana of each node in the network."),
		collector.WithCollectFunc(func() map[string]float64 {
			access := deps.Protocol.Engine().ManaTracker.ManaByIDs()
			res := make(map[string]float64)
			for id, val := range access {
				res[id.String()] = float64(val)
			}
			return res
		}),
	)),
	collector.WithMetric(collector.NewMetric(weightPerNode,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("node_id"),
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
		collector.WithLabels("access", "consensus"),
		collector.WithHelp("Current percentile of the local node in terms of aMana."),
		collector.WithCollectFunc(func() map[string]float64 {
			accessMap := deps.Protocol.Engine().ManaTracker.ManaByIDs()
			accPerc := manamodels.Percentile(deps.Local.ID(), accessMap)
			consensus := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())
			conPerc := manamodels.Percentile(deps.Local.ID(), consensus)

			return collector.MultiValue([]string{"access", "consensus"}, accPerc, conPerc)
		}),
	)),
	collector.WithMetric(collector.NewMetric(neighborsMana,
		collector.WithType(collector.GaugeVec),
		collector.WithLabels("access", "consensus"),
		collector.WithHelp(""),
		collector.WithCollectFunc(func() map[string]float64 {
			neighbors := deps.P2Pmgr.AllNeighbors()
			accessMap := deps.Protocol.Engine().ManaTracker.ManaByIDs()
			consensus := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())

			accAvg := manamodels.NeighborsAverageMana(accessMap, neighbors)
			conAvg := manamodels.NeighborsAverageMana(consensus, neighbors)
			return collector.MultiValue([]string{"access", "consensus"}, accAvg, conAvg)
		}),
	)),
)
