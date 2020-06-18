package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	activeConflicts    prometheus.Gauge
	finalizedConflicts prometheus.Gauge
	failedConflicts    prometheus.Gauge
	avgRoundToFin      prometheus.Gauge
	queryRx            prometheus.Gauge
	queryOpRx          prometheus.Gauge
	queryReplyNotRx    prometheus.Gauge
	queryOpReplyNotRx  prometheus.Gauge
)

func registerFPCMetrics() {
	activeConflicts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_active_conflicts",
		Help: "number of currently active conflicts",
	})
	finalizedConflicts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_finalized_conflicts",
		Help: "number of finalized conflicts since the start of the node",
	})
	failedConflicts = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_failed_conflicts",
		Help: "number of failed conflicts since the start of the node",
	})
	avgRoundToFin = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_avg_rounds_to_finalize",
		Help: "average number of rounds it takes to finalize conflicts since the start of the node",
	})
	queryRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_queries_received",
		Help: "number of received voting queries",
	})
	queryOpRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_queries_opinion_received",
		Help: " number of received opinion queries",
	})
	queryReplyNotRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_query_replies_not_received",
		Help: "number of sent but unanswered queries for conflict opinions",
	})
	queryOpReplyNotRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_query_opinion_replies_not_received",
		Help: " number of opinions that the node failed to gather from peers",
	})

	registry.MustRegister(activeConflicts)
	registry.MustRegister(finalizedConflicts)
	registry.MustRegister(failedConflicts)
	registry.MustRegister(avgRoundToFin)
	registry.MustRegister(queryRx)
	registry.MustRegister(queryOpRx)
	registry.MustRegister(queryReplyNotRx)
	registry.MustRegister(queryOpReplyNotRx)

	addCollect(collectFPCMetrics)
}

func collectFPCMetrics() {
	activeConflicts.Set(float64(metrics.ActiveConflicts()))
	finalizedConflicts.Set(float64(metrics.FinalizedConflict()))
	failedConflicts.Set(float64(metrics.FailedConflicts()))
	avgRoundToFin.Set(metrics.AverageRoundsToFinalize())
	queryRx.Set(float64(metrics.FPCQueryReceived()))
	queryOpRx.Set(float64(metrics.FPCOpinionQueryReceived()))
	queryReplyNotRx.Set(float64(metrics.FPCQueryReplyErrors()))
	queryOpReplyNotRx.Set(float64(metrics.FPCOpinionQueryReplyErrors()))
}
