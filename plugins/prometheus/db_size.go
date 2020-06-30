package prometheus

import (
	"os"
	"path/filepath"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dbSize prometheus.Gauge
)

func registerDBMetrics() {
	dbSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_size_bytes",
			Help: "DB size in bytes.",
		},
	)

	registry.MustRegister(dbSize)

	addCollect(collectDBSize)
}

func collectDBSize() {
	size, err := directorySize(config.Node().GetString(database.CfgDatabaseDir))
	if err == nil {
		dbSize.Set(float64(size))
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
