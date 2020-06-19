package prometheus

import (
	"strconv"

	metricspkg "github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// LIKE conflict outcome
	LIKE = "LIKE"
	// DISLIKE conflict outcome
	DISLIKE = "DISLIKE"
)

var (
	clientsInfoCPU    *prometheus.GaugeVec
	clientsInfoMemory *prometheus.GaugeVec

	clientsNeighborCount *prometheus.GaugeVec
	networkDiameter      prometheus.Gauge

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
		opinionToString(ev.Status),
	).Set(1)

	conflictInitialOpinion.WithLabelValues(
		ev.ConflictID,
		ev.NodeID,
		opinionToString(ev.Opinions[0]),
	).Set(1)
})

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
			Help: "Info about client's neighbors count",
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

	conflictCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conflict_count",
			Help: "Conflicts count labeled with nodeID",
		},
		[]string{
			"nodeID",
		},
	)

	conflictFinalizationRounds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conflict_finalization_rounds",
			Help: "Number of rounds to finalize a given conflict labeled with conflictID and nodeID",
		},
		[]string{
			"conflictID",
			"nodeID",
		},
	)

	conflictInitialOpinion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conflict_initial_opinion",
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
			Name: "conflict_outcome",
			Help: "Outcome of a given conflict labeled with conflictID, nodeID and opinion",
		},
		[]string{
			"conflictID",
			"nodeID",
			"opinion",
		},
	)

	registry.MustRegister(clientsInfoCPU)
	registry.MustRegister(clientsInfoMemory)
	registry.MustRegister(clientsNeighborCount)
	registry.MustRegister(networkDiameter)

	registry.MustRegister(conflictCount)
	registry.MustRegister(conflictFinalizationRounds)
	registry.MustRegister(conflictInitialOpinion)
	registry.MustRegister(conflictOutcome)

	metricspkg.Events().AnalysisFPCFinalized.Attach(onFPCFinalized)

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

func opinionToString(opinion vote.Opinion) string {
	if opinion == vote.Like {
		return LIKE
	}
	return DISLIKE
}
