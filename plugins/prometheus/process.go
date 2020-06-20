package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cpuUsage      prometheus.Gauge
	memUsageBytes prometheus.Gauge
)

func registerProcessMetrics() {
	cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_cpu_usage",
		Help: "CPU (System) usage.",
	})
	memUsageBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "process_mem_usage_bytes",
		Help: "memory usage [bytes].",
	})

	registry.MustRegister(cpuUsage)
	registry.MustRegister(memUsageBytes)

	addCollect(collectProcesskMetrics)
}

func collectProcesskMetrics() {
	cpuUsage.Set(float64(metrics.CPUUsage()))
	memUsageBytes.Set(float64(metrics.MemUsage()))
}
