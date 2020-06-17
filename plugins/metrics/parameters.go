package metrics

import (
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// should always be 1 second
	MPSMeasurementInterval = 1 * time.Second
	TPSMeasurementInterval = 1 * time.Second
	// can be adjusted as wished
	MessageTipsMeasurementInterval = 1 * time.Second
	ValueTipsMeasurementInterval   = 1 * time.Second
	CPUUsageMeasurementInterval    = 1 * time.Second
	MemUsageMeasurementInterval    = 1 * time.Second
	SyncedMeasurementInterval      = 1 * time.Second
)

const (
	// CfgMetricsLocal defines the config flag to enable/disable local metrics.
	CfgMetricsLocal = "metrics.local"
	// CfgMetricsGlobal defines the config flag to enable/disable global metrics.
	CfgMetricsGlobal = "metrics.global"
)

func init() {
	flag.Bool(CfgMetricsLocal, true, "include local metrics")
	flag.Bool(CfgMetricsGlobal, false, "include global metrics")
}
