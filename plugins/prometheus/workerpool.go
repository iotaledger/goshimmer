package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/gossip"
)

var workerpools *prometheus.GaugeVec

func registerWorkerpoolMetrics() {
	workerpools = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workerpools_load",
			Help: "Info about workerpools load",
		},
		[]string{
			"name",
		},
	)

	registry.MustRegister(workerpools)

	addCollect(collectWorkerpoolMetrics)
}

func collectWorkerpoolMetrics() {
	name, load := gossip.Manager().MessageWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))

	name, load = gossip.Manager().MessageRequestWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))
}
