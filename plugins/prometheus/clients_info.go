package prometheus

import (
	"strconv"

	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	clientsInfoCPU    *prometheus.GaugeVec
	clientsInfoMemory *prometheus.GaugeVec
)

func registerClientsMetrics() {
	clientsInfoCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clients_info_cpu",
			Help: "Info about client's cpu load labeled with nodeID, OS, ARCH and number of cpu cores",
		},
		[]string{
			"nodeID",
			"OS",
			"ARCH",
			"NUM_CPU",
		},
	)

	clientsInfoMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clients_info_mem",
			Help: "Info about client's memory usage labeled with nodeID, OS, ARCH and number of cpu cores",
		},
		[]string{
			"nodeID",
			"OS",
			"ARCH",
			"NUM_CPU",
		},
	)

	registry.MustRegister(clientsInfoCPU)
	registry.MustRegister(clientsInfoMemory)

	addCollect(collectClientsInfo)
}

func collectClientsInfo() {
	clientInfoMap := metrics.ClientsMetrics()

	for nodeID, clientInfo := range clientInfoMap {
		clientsInfoCPU.WithLabelValues(
			nodeID,
			clientInfo.OS,
			clientInfo.Arch,
			strconv.Itoa(clientInfo.NumCPU),
		).Set(clientInfo.CPUUsage)
		clientsInfoMemory.WithLabelValues(
			nodeID,
			clientInfo.OS,
			clientInfo.Arch,
			strconv.Itoa(clientInfo.NumCPU),
		).Set(float64(clientInfo.MemoryUsage))
	}
}
