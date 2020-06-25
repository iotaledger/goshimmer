package prometheus

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messageTips         prometheus.Gauge
	messagePerTypeCount *prometheus.GaugeVec
	messageTotalCount   prometheus.Gauge

	transactionCounter prometheus.Gauge
	valueTips          prometheus.Gauge
)

func registerTangleMetrics() {
	messageTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_tips_count",
		Help: "Current number of tips in message tangle",
	})

	messagePerTypeCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_per_type_count",
			Help: "number of messages per payload type seen since the start of the node",
		}, []string{
			"message_type",
		})

	messageTotalCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_count",
		Help: "total number of messages seen since the start of the node",
	})

	transactionCounter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_transaction_counter",
		Help: "number of value transactions (value payloads) seen",
	})

	valueTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_tips",
		Help: "current number of tips in the value tangle",
	})

	registry.MustRegister(messageTips)
	registry.MustRegister(messagePerTypeCount)
	registry.MustRegister(messageTotalCount)
	registry.MustRegister(transactionCounter)
	registry.MustRegister(valueTips)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	messageTips.Set(float64(metrics.MessageTips()))
	msgCountPerPayload := metrics.MessageCountPerPayload()
	for payloadType, count := range msgCountPerPayload {
		messagePerTypeCount.WithLabelValues(convertPayloadTypeToString(payloadType)).Set(float64(count))
	}
	messageTotalCount.Set(float64(metrics.MessageTotalCount()))
	transactionCounter.Set(float64(metrics.ValueTransactionCounter()))
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
