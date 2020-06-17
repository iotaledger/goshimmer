package prometheus

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messagesPerSecond prometheus.Gauge
	messagePerSecondPerPayload *prometheus.GaugeVec
	messageTips prometheus.Gauge
	messageCount *prometheus.GaugeVec
	messageTotalCount prometheus.Gauge

	transactionsPerSecond prometheus.Gauge
	valueTips prometheus.Gauge
)

func init() {
	messagesPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_messages_per_second",
		Help: "Number of messages per second.",
	})

	// TODO: look into why not work
	messagePerSecondPerPayload = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_per_second_per_payload",
			Help: " message payload types and their corresponding MPS values",
		}, []string{
			"message_type",
		})

	// TODO: look into why not work
	messageTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_tips_count",
		Help: "Current number of tips in message tangle",
	})

	messageCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_count",
			Help: "number of messages seen since the start of the node",
		}, []string{
			"message_type",
		})

	messageTotalCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_count",
		Help: "total number of messages seen since the start of the node",
	})

	transactionsPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_transaction_per_second",
		Help: "current transactions (value payloads) per second number",
	})

	valueTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_tips",
		Help: "current number of tips in the value tangle",
	})

	registry.MustRegister(messagesPerSecond)
	registry.MustRegister(messagePerSecondPerPayload)
	registry.MustRegister(messageTips)
	registry.MustRegister(messageCount)
	registry.MustRegister(messageTotalCount)
	registry.MustRegister(transactionsPerSecond)
	registry.MustRegister(valueTips)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	messagesPerSecond.Set(float64(metrics.MPS()))
	msgMPSPerPayload := metrics.MPSPerPayload()
	for payloadType, mps := range msgMPSPerPayload {
		messagePerSecondPerPayload.WithLabelValues(convertPayloadTypeToString(payloadType)).Set(float64(mps))
	}
	messageTips.Set(float64(metrics.MessageTips()))
	msgCountPerPayload := metrics.MessageCountPerPayload()
	for payloadType, count := range msgCountPerPayload {
		messageCount.WithLabelValues(convertPayloadTypeToString(payloadType)).Set(float64(count))
	}
	messageTotalCount.Set(float64(metrics.MessageTotalCount()))
	transactionsPerSecond.Set(float64(metrics.TPS()))
	valueTips.Set(float64(metrics.ValueTips()))
}

func convertPayloadTypeToString(p payload.Type) string {
	switch p {
	case 0:
		return "data"
	case 1:
		return "value"
	case 111:
		return "drng"
	default:
		return "unknown"
	}
}
