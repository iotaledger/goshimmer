package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var dbSizes *prometheus.GaugeVec

func registerDBMetrics() {
	dbSizes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "db",
			Name:      "size",
			Help:      "DB size in bytes.",
		},
		[]string{
			"type",
		},
	)

	registry.MustRegister(dbSizes)

	addCollect(collectStorageDBSize)
	if deps.Retainer != nil {
		addCollect(collectRetainerDBSize)
	}
}

func collectStorageDBSize() {
	dbSizes.WithLabelValues("storage_permanent").Set(float64(deps.Protocol.MainStorage().PermanentDatabaseSize()))
	dbSizes.WithLabelValues("storage_prunable").Set(float64(deps.Protocol.MainStorage().PrunableDatabaseSize()))
}

func collectRetainerDBSize() {
	dbSizes.WithLabelValues("retainer").Set(float64(deps.Retainer.DatabaseSize()))
}
