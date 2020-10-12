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
	averageNeighborsAccess    prometheus.Gauge
	averageNeighborsConsensus prometheus.Gauge
	averageAccessPledgeBM1    *prometheus.GaugeVec
	averageConsensusPledgeBM1 *prometheus.GaugeVec
	averageAccessPledgeBM2    *prometheus.GaugeVec
	averageConsensusPledgeBM2 *prometheus.GaugeVec
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

	averageAccessPledgeBM1 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_access_pledge_bm1",
			Help: "Average base mana 1 of pledged access mana.",
		},
		[]string{
			"nodeID",
		})
	averageAccessPledgeBM2 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_access_pledge_bm2",
			Help: "Average base mana 2 of pledged access mana.",
		},
		[]string{
			"nodeID",
		})

	averageConsensusPledgeBM1 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_consensus_pledge_bm1",
			Help: "Average base mana 1 of pledged consensus mana.",
		},
		[]string{
			"nodeID",
		})
	averageConsensusPledgeBM2 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_average_consensus_pledge_bm2",
			Help: "Average base mana 2 of pledged consensus mana.",
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
	registry.MustRegister(averageAccessPledgeBM1)
	registry.MustRegister(averageConsensusPledgeBM1)
	registry.MustRegister(averageAccessPledgeBM2)
	registry.MustRegister(averageConsensusPledgeBM2)
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
	averageNeighborsAccess.Set(metrics.AverageNeighborsAccess())
	averageNeighborsConsensus.Set(metrics.AverageNeighborsConsensus())

	accessBM1Pledges, accessBM2Pledges := metrics.AveragePledgeAccessBM()
	for nodeID, value := range accessBM1Pledges {
		averageAccessPledgeBM1.WithLabelValues(nodeID.String()).Set(value)
	}
	for nodeID, value := range accessBM2Pledges {
		averageAccessPledgeBM2.WithLabelValues(nodeID.String()).Set(value)
	}

	consensusBM1Pledges, consensusBM2Pledges := metrics.AveragePledgeConsensusBM()
	for nodeID, value := range consensusBM1Pledges {
		averageConsensusPledgeBM1.WithLabelValues(nodeID.String()).Set(value)
	}
	for nodeID, value := range consensusBM2Pledges {
		averageConsensusPledgeBM2.WithLabelValues(nodeID.String()).Set(value)
	}
}
