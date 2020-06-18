package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	clientsInfo *prometheus.GaugeVec
)

func registerClientsMetrics() {
	clientsInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clients_info",
			Help: "Clients info.",
		},
		[]string{"info"},
		// []string{"OS", "ARCH", "NUM_CPU", "CPU_USAGE", "MEM_USAGE"},
	)

	// registry.MustRegister(clientsInfo)

	// collectClientsInfo()

	// clientsInfo.WithLabelValues(banner.AppName, banner.AppVersion).Set(1)
}

func collectClientsInfo() {
	// for ID, info := range metrics.ClientsMetrics() {
	// 	clientsInfo.WithLabelValues()
	// }

}
