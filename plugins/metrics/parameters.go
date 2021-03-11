package metrics

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgMetricsLocal defines the config flag to enable/disable local metrics.
	CfgMetricsLocal = "metrics.local"
	// CfgMetricsGlobal defines the config flag to enable/disable global metrics.
	CfgMetricsGlobal = "metrics.global"
	// CfgManaUpdateInterval defines the interval in seconds at which mana metrics are updated.
	CfgManaUpdateInterval = "metrics.manaUpdateInterval"
	// CfgMetricsManaResearch defines mana metrics for research.
	CfgMetricsManaResearch = "metrics.manaResearch"
)

func init() {
	flag.Bool(CfgMetricsLocal, true, "include local metrics")
	flag.Bool(CfgMetricsGlobal, false, "include global metrics")
	flag.Int(CfgManaUpdateInterval, 30, "mana metrics update interval")
	flag.Bool(CfgMetricsManaResearch, false, "include research mana metrics")
}
