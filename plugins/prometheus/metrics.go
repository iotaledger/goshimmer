package prometheus

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messagesPerSecond prometheus.Gauge
	infoMessageTips   prometheus.Gauge
	infoValueTips     prometheus.Gauge
)

func init() {
	messagesPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_messages_per_second",
		Help: "Number of messages per second.",
	})
	infoMessageTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_message_tips",
		Help: "Number of message tips.",
	})
	infoValueTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_info_value_tips",
		Help: "Number of value tips.",
	})

	registry.MustRegister(messagesPerSecond)
	registry.MustRegister(infoApp)
	registry.MustRegister(infoMessageTips)

	addCollect(collectMetrics)
}

func collectMetrics() {
	// MPS
	messagesPerSecond.Set(float64(metrics.ReceivedMessagesPerSecond()))

	// Tips
	infoMessageTips.Set(float64(messagelayer.TipSelector.TipCount()))
	infoValueTips.Set(float64(valuetransfers.TipManager().Size()))
}
