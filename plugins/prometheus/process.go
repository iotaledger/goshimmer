package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
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

	addCollect(collectProcessMetrics)
}

func collectProcessMetrics() {
	cpuUsage.Set(metrics.CPUUsage())
	memUsageBytes.Set(float64(metrics.MemUsage()))
}
