package prometheus

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgPrometheusGoMetrics defines the config flag to enable/disable go metrics.
	CfgPrometheusGoMetrics = "prometheus.goMetrics"
	// CfgPrometheusProcessMetrics defines the config flag to enable/disable process metrics.
	CfgPrometheusProcessMetrics = "prometheus.processMetrics"
	// CfgPrometheusPromhttpMetrics defines the config flag to enable/disable promhttp metrics.
	CfgPrometheusPromhttpMetrics = "prometheus.promhttpMetrics"
	// CfgPrometheusWorkerpoolMetrics defines the config flag to enable/disable workerpool metrics.
	CfgPrometheusWorkerpoolMetrics = "prometheus.workerpoolMetrics"
	// CfgPrometheusBindAddress defines the config flag of the bind address on which the Prometheus exporter listens on.
	CfgPrometheusBindAddress = "prometheus.bindAddress"
)

func init() {
	flag.String(CfgPrometheusBindAddress, "0.0.0.0:9311", "the bind address on which the Prometheus exporter listens on")
	flag.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
	flag.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
	flag.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
	flag.Bool(CfgPrometheusWorkerpoolMetrics, false, "include workerpool metrics")
}
