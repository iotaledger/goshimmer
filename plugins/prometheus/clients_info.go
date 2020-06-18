package prometheus

import (
	"strconv"

	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	clientsInfoCPU    *prometheus.GaugeVec
	clientsInfoMemory *prometheus.GaugeVec

	clientsNeighborCount *prometheus.GaugeVec
	networkDiameter      prometheus.Gauge
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

	clientsNeighborCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "clients_neighbor_count",
			Help: "Info about client's memory usage labeled with nodeID, OS, ARCH and number of cpu cores",
		},
		[]string{
			"nodeID",
			"direction",
		},
	)

	networkDiameter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "clients_network_diameter",
		Help: "Autopeering network diameter",
	})

	registry.MustRegister(clientsInfoCPU)
	registry.MustRegister(clientsInfoMemory)
	registry.MustRegister(clientsNeighborCount)
	registry.MustRegister(networkDiameter)

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

	for nodeID, neighborCount := range analysisdashboard.NumOfNeighbors() {
		clientsNeighborCount.WithLabelValues(nodeID, "in").Set(float64(neighborCount.Inbound))
		clientsNeighborCount.WithLabelValues(nodeID, "out").Set(float64(neighborCount.Inbound))
	}
	networkDiameter.Set(float64(metrics.NetworkDiameter()))
}
