package prometheus

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgPrometheusGoMetrics       = "prometheus.goMetrics"
	CfgPrometheusProcessMetrics  = "prometheus.processMetrics"
	CfgPrometheusPromhttpMetrics = "prometheus.promhttpMetrics"
	CfgPrometheusBindAddress     = "prometheus.bindAddress"
)

func init() {
	flag.String(CfgPrometheusBindAddress, "localhost:9311", "the bind address on which the Prometheus exporter listens on")
	flag.Bool(CfgPrometheusGoMetrics, false, "include go metrics")
	flag.Bool(CfgPrometheusProcessMetrics, false, "include process metrics")
	flag.Bool(CfgPrometheusPromhttpMetrics, false, "include promhttp metrics")
}
