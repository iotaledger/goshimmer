package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	accessManaMap             *prometheus.GaugeVec
	accessPercentile          prometheus.Gauge
	consensusManaMap          *prometheus.GaugeVec
	consensusPercentile       prometheus.Gauge
	averageAccessPledgeMap    *prometheus.GaugeVec
	averageConsensusPledgeMap *prometheus.GaugeVec
	averageNeighborsAccess    prometheus.Gauge
	averageNeighborsConsensus prometheus.Gauge
)

func registerManaMetrics() {
	accessManaMap = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_access",
			Help: "Access mana map of the network",
		},
		[]string{
			"nodeID",
		})

	accessPercentile = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_access_percentile",
			Help: "Top percentile node belongs to in terms of access mana.",
		})

	consensusManaMap = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_consensus",
			Help: "Consensus mana map of the network",
		},
		[]string{
			"nodeID",
		})

	consensusPercentile = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_consensus_percentile",
			Help: "Top percentile node belongs to in terms of consensus mana.",
		})

	averageAccessPledgeMap = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_access_pledge",
			Help: "Average pledged access mana of nodes",
		},
		[]string{
			"nodeID",
		})

	averageConsensusPledgeMap = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_consensus_pledge",
			Help: "Average pledged consensus mana of nodes",
		},
		[]string{
			"nodeID",
		})

	averageNeighborsAccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_average_neighbors_access",
			Help: "Average access mana of all neighbors.",
		})

	averageNeighborsConsensus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_average_neighbors_consensus",
			Help: "Average consensus mana of all neighbors.",
		})

	registry.MustRegister(accessManaMap)
	registry.MustRegister(accessPercentile)
	registry.MustRegister(consensusManaMap)
	registry.MustRegister(consensusPercentile)
	registry.MustRegister(averageAccessPledgeMap)
	registry.MustRegister(averageConsensusPledgeMap)
	registry.MustRegister(averageNeighborsAccess)
	registry.MustRegister(averageNeighborsConsensus)

	addCollect(collectManaMetrics)
}

func collectManaMetrics() {
	for nodeID, value := range metrics.AccessManaMap() {
		accessManaMap.WithLabelValues(nodeID.String()).Set(value)
	}
	accessPercentile.Set(metrics.AccessPercentile())
	for nodeID, value := range metrics.ConsensusManaMap() {
		consensusManaMap.WithLabelValues(nodeID.String()).Set(value)
	}
	consensusPercentile.Set(metrics.ConsensusPercentile())
	for nodeID, value := range metrics.AveragePledgeAccessMap() {
		averageAccessPledgeMap.WithLabelValues(nodeID.String()).Set(value)
	}
	for nodeID, value := range metrics.AveragePledgeConsensusMap() {
		averageConsensusPledgeMap.WithLabelValues(nodeID.String()).Set(value)
	}
	averageNeighborsAccess.Set(metrics.AverageNeighborsAccess())
	averageNeighborsConsensus.Set(metrics.AverageNeighborsConsensus())
}
