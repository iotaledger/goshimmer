package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	neighborDropCount             prometheus.Gauge
	avgNeighborConnectionLifeTime prometheus.Gauge
	connectionsCount              prometheus.Gauge
	minDistance                   prometheus.Gauge
	maxDistance                   prometheus.Gauge
	avgDistance                   prometheus.Gauge
)

func registerAutopeeringMetrics() {
	neighborDropCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_neighbor_drop_count",
		Help: "Autopeering neighbor drop count.",
	})

	avgNeighborConnectionLifeTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_avg_neighbor_connection_lifetime",
		Help: "Autopeering average neighbor connection lifetime.",
	})

	connectionsCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_neighbor_connections_count",
		Help: "Autopeering neighbor connections count.",
	})

	minDistance = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_min_distance",
		Help: "Autopeering minimum distance with all neighbors.",
	})

	maxDistance = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_max_distance",
		Help: "Autopeering maximum distance with all neighbors.",
	})

	avgDistance = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_avg_distance",
		Help: "Autopeering average distance with all neighbors.",
	})

	registry.MustRegister(neighborDropCount)
	registry.MustRegister(avgNeighborConnectionLifeTime)
	registry.MustRegister(connectionsCount)
	registry.MustRegister(minDistance)
	registry.MustRegister(maxDistance)
	registry.MustRegister(avgDistance)

	addCollect(collectAutopeeringMetrics)
}

func collectAutopeeringMetrics() {
	neighborDropCount.Set(float64(metrics.NeighborDropCount()))
	avgNeighborConnectionLifeTime.Set(metrics.AvgNeighborConnectionLifeTime())
	connectionsCount.Set(float64(metrics.NeighborConnectionsCount()))
	min, max, avg := metrics.AutopeeringDistanceStats()
	minDistance.Set(float64(min))
	maxDistance.Set(float64(max))
	avgDistance.Set(avg)
}
