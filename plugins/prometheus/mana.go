package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	accessMana    *prometheus.GaugeVec
	consensusMana *prometheus.GaugeVec
)

func registerManaMetrics() {
	accessMana = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_access",
			Help: "Access mana map of the network",
		},
		[]string{
			"nodeID",
		})

	consensusMana = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mana_consensus",
			Help: "Consensus mana map of the network",
		},
		[]string{
			"nodeID",
		})

	registry.MustRegister(accessMana)
	registry.MustRegister(consensusMana)

	addCollect(collectManaMetrics)
}

func collectManaMetrics() {
	for nodeID, value := range metrics.AccessMana() {
		accessMana.WithLabelValues(nodeID.String()).Set(value)
	}
	for nodeID, value := range metrics.ConsensusMana() {
		consensusMana.WithLabelValues(nodeID.String()).Set(value)
	}
}
