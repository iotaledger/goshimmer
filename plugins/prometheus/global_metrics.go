package prometheus

import (
	"strconv"

	metricspkg "github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	like    = "LIKE"
	dislike = "DISLIKE"
)

// These metrics store information collected via the analysis server.
var (
	// Process related metrics.
	nodesInfoCPU    *prometheus.GaugeVec
	nodesInfoMemory *prometheus.GaugeVec

	// Autopeering related metrics.
	nodesNeighborCount *prometheus.GaugeVec
	networkDiameter    prometheus.Gauge

	// FPC related metrics.
	conflictCount              *prometheus.GaugeVec
	conflictFinalizationRounds *prometheus.GaugeVec
	conflictOutcome            *prometheus.GaugeVec
	conflictInitialOpinion     *prometheus.GaugeVec
)

var onFPCFinalized = events.NewClosure(func(ev *metricspkg.AnalysisFPCFinalizedEvent) {
	conflictCount.WithLabelValues(
		ev.NodeID,
	).Add(1)

	conflictFinalizationRounds.WithLabelValues(
		ev.ConflictID,
		ev.NodeID,
	).Set(float64(ev.Rounds + 1))

	conflictOutcome.WithLabelValues(
		ev.ConflictID,
		ev.NodeID,
		opinionToString(ev.Outcome),
	).Set(1)

	if len(ev.Opinions) > 0 {
		conflictInitialOpinion.WithLabelValues(
			ev.ConflictID,
			ev.NodeID,
			opinionToString(ev.Opinions[0]),
		).Set(1)
	}
})

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

	conflictCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_conflict_count",
			Help: "Conflicts count labeled with nodeID",
		},
		[]string{
			"nodeID",
		},
	)

	conflictFinalizationRounds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_conflict_finalization_rounds",
			Help: "Number of rounds to finalize a given conflict labeled with conflictID and nodeID",
		},
		[]string{
			"conflictID",
			"nodeID",
		},
	)

	conflictInitialOpinion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_conflict_initial_opinion",
			Help: "Initial opinion of a given conflict labeled with conflictID, nodeID and opinion",
		},
		[]string{
			"conflictID",
			"nodeID",
			"opinion",
		},
	)

	conflictOutcome = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "global_conflict_outcome",
			Help: "Outcome of a given conflict labeled with conflictID, nodeID and opinion",
		},
		[]string{
			"conflictID",
			"nodeID",
			"opinion",
		},
	)

	registry.MustRegister(nodesInfoCPU)
	registry.MustRegister(nodesInfoMemory)
	registry.MustRegister(nodesNeighborCount)
	registry.MustRegister(networkDiameter)

	registry.MustRegister(conflictCount)
	registry.MustRegister(conflictFinalizationRounds)
	registry.MustRegister(conflictInitialOpinion)
	registry.MustRegister(conflictOutcome)

	metricspkg.Events().AnalysisFPCFinalized.Attach(onFPCFinalized)

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

func opinionToString(o opinion.Opinion) string {
	if o == opinion.Like {
		return like
	}
	return dislike
}
