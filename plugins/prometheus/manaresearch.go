package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	accessManaMapResearch       *prometheus.GaugeVec
	accessPercentileResearch    *prometheus.GaugeVec
	consensusManaMapResearch    *prometheus.GaugeVec
	consensusPercentileResearch *prometheus.GaugeVec
)

func registerManaResearchMetrics() {
	accessManaMapResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_access",
			Help: "Access mana map of the network",
		},
		[]string{
			"nodeID",
			"method",
		})

	accessPercentileResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_access_percentile",
			Help: "Top percentile node belongs to in terms of access mana.",
		},
		[]string{
			"method",
		})

	consensusManaMapResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_consensus",
			Help: "Consensus mana map of the network",
		},
		[]string{
			"nodeID",
			"method",
		})

	consensusPercentileResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_consensus_percentile",
			Help: "Top percentile node belongs to in terms of consensus mana.",
		},
		[]string{
			"method",
		})

	registry.MustRegister(accessManaMapResearch)
	registry.MustRegister(accessPercentileResearch)
	registry.MustRegister(consensusManaMapResearch)
	registry.MustRegister(consensusPercentileResearch)

	addCollect(collectManaResearchMetrics)
}

func collectManaResearchMetrics() {
	for nodeID, value := range metrics.AccessManaMapBM1() {
		accessManaMapResearch.WithLabelValues(nodeID.String(), "bm1").Set(value)
	}
	for nodeID, value := range metrics.AccessManaMapBM2() {
		accessManaMapResearch.WithLabelValues(nodeID.String(), "bm2").Set(value)
	}
	accessPercentileResearch.WithLabelValues("bm1").Set(metrics.AccessPercentileBM1())
	accessPercentileResearch.WithLabelValues("bm2").Set(metrics.AccessPercentileBM2())
	for nodeID, value := range metrics.ConsensusManaMapBM1() {
		consensusManaMapResearch.WithLabelValues(nodeID.String(), "bm1").Set(value)
	}
	for nodeID, value := range metrics.ConsensusManaMapBM2() {
		consensusManaMapResearch.WithLabelValues(nodeID.String(), "bm2").Set(value)
	}
	consensusPercentileResearch.WithLabelValues("bm1").Set(metrics.ConsensusPercentileBM1())
	consensusPercentileResearch.WithLabelValues("bm2").Set(metrics.ConsensusPercentileBM2())
}
