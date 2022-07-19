package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	rateSetterRate       prometheus.Gauge
	rateSetterBufferSize prometheus.Gauge
	rateSetterEstimate   prometheus.Gauge
)

func registerRateSetterMetrics() {
	rateSetterRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ratesetter_own_rate",
		Help: "node's own rate (blocks per second).",
	})

	rateSetterBufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ratesetter_buffer_size",
		Help: "number of ready blocks in the scheduler buffer.",
	})

	rateSetterEstimate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ratesetter_estimate",
		Help: "number of milliseconds until a new block will be issued by the node.",
	})
	registry.MustRegister(rateSetterRate)
	registry.MustRegister(rateSetterBufferSize)
	registry.MustRegister(rateSetterEstimate)

	addCollect(collectRateSetterMetrics)
}

func collectRateSetterMetrics() {
	rateSetterRate.Set(metrics.OwnRate())
	rateSetterBufferSize.Set(float64(metrics.RateSetterBufferSize()))
	rateSetterEstimate.Set(float64(metrics.RateSetterEstimate()))
}
