package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messagesPerSecond prometheus.Gauge
)

func init() {
	messagesPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_messages_per_second",
		Help: "Number of messages per second.",
	})

	registry.MustRegister(messagesPerSecond)

	addCollect(collectMetrics)
}

func collectMetrics() {
	messagesPerSecond.Set(float64(metrics.ReceivedMessagesPerSecond()))
}
