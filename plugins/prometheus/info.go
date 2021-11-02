package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	infoApp              *prometheus.GaugeVec
	tangleTimeSyncStatus prometheus.Gauge
	nodeID               string
)

func registerInfoMetrics() {
	infoApp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_app",
			Help: "Node software name and version.",
		},
		[]string{"name", "version", "nodeID"},
	)
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}
	infoApp.WithLabelValues(banner.AppName, banner.AppVersion, nodeID).Set(1)

	tangleTimeSyncStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangleTimeSynced",
		Help: "Node sync status based on TangleTime.",
	})

	registry.MustRegister(infoApp)
	registry.MustRegister(tangleTimeSyncStatus)

	addCollect(collectInfoMetrics)
}

func collectInfoMetrics() {
	tangleTimeSyncStatus.Set(func() float64 {
		if metrics.TangleTimeSynced() {
			return 1
		}
		return 0
	}())
}
