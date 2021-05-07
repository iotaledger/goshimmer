package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	infoApp              *prometheus.GaugeVec
	tangleTimeSyncStatus prometheus.Gauge
	syncBeaconSyncStatus prometheus.Gauge

	nodeID string
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

	tangleTimeSyncStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangleTimeSynced",
		Help: "Node sync status based on TangleTime.",
	})

	syncBeaconSyncStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "syncBeaconSynced",
		Help: "Node sync status based on SyncBeacon.",
	})

	registry.MustRegister(infoApp)
	registry.MustRegister(tangleTimeSyncStatus)
	registry.MustRegister(syncBeaconSyncStatus)

	addCollect(collectInfoMetrics)
}

func collectInfoMetrics() {
	tangleTimeSyncStatus.Set(func() float64 {
		if metrics.TangleTimeSynced() {
			return 1
		}
		return 0
	}())

	syncBeaconSyncStatus.Set(func() float64 {
		if metrics.SyncBeaconSynced() {
			return 1
		}
		return 0
	}())
}
