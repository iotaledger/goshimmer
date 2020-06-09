package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	infoApp  *prometheus.GaugeVec
	infoTips prometheus.Gauge
)

func init() {
	infoApp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_app",
			Help: "Node software name and version.",
		},
		[]string{"name", "version"},
	)
	infoTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_tips",
		Help: "Number of tips.",
	})

	infoApp.WithLabelValues(banner.AppName, banner.AppVersion).Set(1)
	registry.MustRegister(infoApp)
	registry.MustRegister(infoTips)
	addCollect(collectInfo)
}

func collectInfo() {
	// Tips
	infoTips.Set(0)
}
