package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	accessManaMap             *prometheus.GaugeVec
	accessPercentile          prometheus.Gauge
	consensusManaMap          *prometheus.GaugeVec
	consensusPercentile       prometheus.Gauge
	averageNeighborsAccess    prometheus.Gauge
	averageNeighborsConsensus prometheus.Gauge
	averageAccessPledge       *prometheus.GaugeVec
	averageConsensusPledge    *prometheus.GaugeVec
	delegatedMana             prometheus.Gauge
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

	averageAccessPledge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_access_pledge",
			Help: "Average of pledged access mana.",
		},
		[]string{
			"nodeID",
			"type",
		})

	averageConsensusPledge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_consensus_pledge",
			Help: "Average of pledged consensus mana.",
		},
		[]string{
			"nodeID",
			"type",
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

	delegatedMana = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_delegated_amount",
			Help: "The amount of mana delegated to the node at the moment",
		})

	registry.MustRegister(accessManaMap)
	registry.MustRegister(accessPercentile)
	registry.MustRegister(consensusManaMap)
	registry.MustRegister(consensusPercentile)
	registry.MustRegister(averageAccessPledge)
	registry.MustRegister(averageConsensusPledge)
	registry.MustRegister(averageNeighborsAccess)
	registry.MustRegister(averageNeighborsConsensus)
	registry.MustRegister(delegatedMana)

	addCollect(collectManaMetrics)
}

func collectManaMetrics() {
	accessManaMap.Reset()
	for nodeID, value := range metrics.AccessManaMap() {
		accessManaMap.WithLabelValues(nodeID.String()).Set(value)
	}
	accessPercentile.Set(metrics.AccessPercentile())
	consensusManaMap.Reset()
	for nodeID, value := range metrics.ConsensusManaMap() {
		consensusManaMap.WithLabelValues(nodeID.String()).Set(value)
	}
	consensusPercentile.Set(metrics.ConsensusPercentile())
	averageNeighborsAccess.Set(metrics.AverageNeighborsAccess())
	averageNeighborsConsensus.Set(metrics.AverageNeighborsConsensus())

	accessPledges := metrics.AveragePledgeAccess()
	averageAccessPledge.Reset()
	for nodeID, value := range accessPledges {
		averageAccessPledge.WithLabelValues(nodeID.String(), "bm2").Set(value)
	}

	consensusPledges := metrics.AveragePledgeConsensus()
	averageConsensusPledge.Reset()
	for nodeID, value := range consensusPledges {
		averageConsensusPledge.WithLabelValues(nodeID.String(), "bm1").Set(value)
	}

	delegatedMana.Set(float64(metrics.DelegatedMana()))
}
