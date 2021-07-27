package prometheus

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

// These metrics store information collected via the analysis server.
var (
	// Process related metrics.
	nodesInfoCPU    *prometheus.GaugeVec
	nodesInfoMemory *prometheus.GaugeVec

	// Autopeering related metrics.
	nodesNeighborCount *prometheus.GaugeVec
	networkDiameter    prometheus.Gauge
)

func registerClientsMetrics() {
	nodesInfoCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_nodes_info_cpu",
			Help: "Info about node's cpu load labeled with nodeID, OS, ARCH and number of cpu cores",
		},
		[]string{
			"nodeID",
			"OS",
			"ARCH",
			"NUM_CPU",
		},
	)

	nodesInfoMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_nodes_info_mem",
			Help: "Info about node's memory usage labeled with nodeID, OS, ARCH and number of cpu cores",
		},
		[]string{
			"nodeID",
			"OS",
			"ARCH",
			"NUM_CPU",
		},
	)

	nodesNeighborCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_nodes_neighbor_count",
			Help: "Info about node's neighbors count",
		},
		[]string{
			"nodeID",
			"direction",
		},
	)

	networkDiameter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "global_network_diameter",
		Help: "Autopeering network diameter",
	})

	registry.MustRegister(nodesInfoCPU)
	registry.MustRegister(nodesInfoMemory)
	registry.MustRegister(nodesNeighborCount)
	registry.MustRegister(networkDiameter)

	addCollect(collectNodesInfo)
}

func collectNodesInfo() {
	nodeInfoMap := metrics.NodesMetrics()

	for nodeID, nodeMetrics := range nodeInfoMap {
		nodesInfoCPU.WithLabelValues(
			nodeID,
			nodeMetrics.OS,
			nodeMetrics.Arch,
			strconv.Itoa(nodeMetrics.NumCPU),
		).Set(nodeMetrics.CPUUsage)

		nodesInfoMemory.WithLabelValues(
			nodeID,
			nodeMetrics.OS,
			nodeMetrics.Arch,
			strconv.Itoa(nodeMetrics.NumCPU),
		).Set(float64(nodeMetrics.MemoryUsage))
	}

	// TODO: send data for all available networkIDs, not just current
	if analysisserver.Networks[banner.SimplifiedAppVersion] != nil {
		for nodeID, neighborCount := range analysisserver.Networks[banner.SimplifiedAppVersion].NumOfNeighbors() {
			nodesNeighborCount.WithLabelValues(nodeID, "in").Set(float64(neighborCount.Inbound))
			nodesNeighborCount.WithLabelValues(nodeID, "out").Set(float64(neighborCount.Outbound))
		}
	}

	networkDiameter.Set(float64(metrics.NetworkDiameter()))
}
