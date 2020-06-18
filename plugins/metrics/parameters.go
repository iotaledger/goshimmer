package metrics

import (
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// should always be 1 second

	// MPSMeasurementInterval defines the MPS measurement interval
	MPSMeasurementInterval = 1 * time.Second
	// TPSMeasurementInterval defines the TPS measurement interval
	TPSMeasurementInterval = 1 * time.Second

	// can be adjusted as wished

	// MessageTipsMeasurementInterval defines the Communication-layer tips measurement interval
	MessageTipsMeasurementInterval = 1 * time.Second
	// ValueTipsMeasurementInterval defines the Value-layer tips measurement interval
	ValueTipsMeasurementInterval = 1 * time.Second
	// CPUUsageMeasurementInterval defines the CPU usage measurement interval
	CPUUsageMeasurementInterval = 1 * time.Second
	// MemUsageMeasurementInterval defines the mem usage measurement interval
	MemUsageMeasurementInterval = 1 * time.Second
	// SyncedMeasurementInterval defines the synced measurement interval
	SyncedMeasurementInterval = 1 * time.Second
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
