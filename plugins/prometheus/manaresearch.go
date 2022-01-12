package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	accessManaMapResearch       *prometheus.GaugeVec
	accessPercentileResearch    prometheus.Gauge
	consensusManaMapResearch    *prometheus.GaugeVec
	consensusPercentileResearch prometheus.Gauge
)

func registerManaResearchMetrics() {
	accessManaMapResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_access",
			Help: "Access mana map of the network",
		},
		[]string{
			"nodeID",
		})

	accessPercentileResearch = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_research_access_percentile",
			Help: "Top percentile node belongs to in terms of access mana.",
		})

	consensusManaMapResearch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_research_consensus",
			Help: "Consensus mana map of the network",
		},
		[]string{
			"nodeID",
		})

	consensusPercentileResearch = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mana_research_consensus_percentile",
			Help: "Top percentile node belongs to in terms of consensus mana.",
		})

	registry.MustRegister(accessManaMapResearch)
	registry.MustRegister(accessPercentileResearch)
	registry.MustRegister(consensusManaMapResearch)
	registry.MustRegister(consensusPercentileResearch)

	addCollect(collectManaResearchMetrics)
}

func collectManaResearchMetrics() {
	accessManaMapResearch.Reset()
	for nodeID, value := range metrics.AccessResearchManaMap() {
		accessManaMapResearch.WithLabelValues(nodeID.String()).Set(value)
	}
	accessPercentileResearch.Set(metrics.AccessResearchPercentile())
	consensusManaMapResearch.Reset()
	for nodeID, value := range metrics.ConsensusResearchManaMap() {
		consensusManaMapResearch.WithLabelValues(nodeID.String()).Set(value)
	}
	consensusPercentileResearch.Set(metrics.ConsensusResearchPercentile())
}
