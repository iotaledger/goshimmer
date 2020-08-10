package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	infoApp *prometheus.GaugeVec
	sync    prometheus.Gauge
	nodeID  string
)

func registerInfoMetrics() {
	infoApp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_app",
			Help: "Node software name and version.",
		},
		[]string{"name", "version", "nodeID"},
	)
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}
	infoApp.WithLabelValues(banner.AppName, banner.AppVersion, nodeID).Set(1)

	sync = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sync",
		Help: "Node sync status.",
	})

	registry.MustRegister(infoApp)
	registry.MustRegister(sync)

	addCollect(collectInfoMetrics)
}

func collectInfoMetrics() {
	sync.Set(func() float64 {
		if metrics.Synced() {
			return 1.0
		}
		return 0.
	}())
}
