package metrics

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgMetricsLocal defines the config flag to enable/disable local metrics.
	CfgMetricsLocal = "metrics.local.enable"
	// CfgMetricsLocalDB defines the config flag to enable/disable local database metrics.
	CfgMetricsLocalDB = "metrics.local.db"
	// CfgMetricsGlobal defines the config flag to enable/disable global metrics.
	CfgMetricsGlobal = "metrics.global.enable"
)

func init() {
	flag.Bool(CfgMetricsLocal, true, "include local metrics")
	flag.Bool(CfgMetricsLocalDB, false, "include local database metrics")
	flag.Bool(CfgMetricsGlobal, false, "include global metrics")
}
