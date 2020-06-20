package prometheus

import (
	"os"
	"path/filepath"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dbSizes *prometheus.GaugeVec
)

func registerDBMetrics() {
	dbSizes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_size_bytes",
			Help: "DB size in bytes.",
		},
		[]string{"name"},
	)

	registry.MustRegister(dbSizes)

	addCollect(colectDBSize)
}

func colectDBSize() {
	dbSizes.Reset()
	dbSize, err := directorySize(config.Node.GetString(database.CfgDatabaseDir))
	if err == nil {
		dbSizes.WithLabelValues("database").Set(float64(dbSize))
	}
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
