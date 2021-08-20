package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
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

	if deps.GossipMgr != nil {
		addCollect(collectWorkerpoolMetrics)
	}
}

func collectWorkerpoolMetrics() {
	name, load := deps.GossipMgr.MessageWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))

	name, load = deps.GossipMgr.MessageRequestWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))
}
