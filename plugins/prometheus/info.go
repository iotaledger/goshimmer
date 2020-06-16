package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	infoApp *prometheus.GaugeVec
)

func init() {
	infoApp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_info_app",
			Help: "Node software name and version.",
		},
		[]string{"name", "version"},
	)
	infoApp.WithLabelValues(banner.AppName, banner.AppVersion).Set(1)
}
