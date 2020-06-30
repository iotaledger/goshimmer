package metrics

import (
	flag "github.com/spf13/pflag"
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
