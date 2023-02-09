package metrics

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
)

const (
	dbNamespace = "db"

	sizeBytes = "size_bytes"

	storagePermanentSizeLabel = "storage_permanent"
	storagePrunableSizeLabel  = "storage_prunable"
	retainerSizeLabel         = "retainer"
)

var DBMetrics = collector.NewCollection(dbNamespace,
	collector.WithMetric(collector.NewMetric(sizeBytes,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("DB size in bytes for permanent, prunable storage and retainer plugin"),
		collector.WithLabels("type"),
		collector.WithCollectFunc(func() map[string]float64 {
			mainEngine := deps.Protocol.MainEngineInstance()
			return collector.MultiLabelsValues(
				[]string{storagePermanentSizeLabel, storagePrunableSizeLabel, retainerSizeLabel},
				mainEngine.Storage.PermanentDatabaseSize(),
				mainEngine.Storage.PrunableDatabaseSize(),
				deps.Retainer.DatabaseSize(),
			)
		}),
	)),
)
