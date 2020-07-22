package prometheus

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messageTips           prometheus.Gauge
	messagePerTypeCount   *prometheus.GaugeVec
	messageTotalCount     prometheus.Gauge
	messageTotalCountDB   prometheus.Gauge
	messageSolidCountDB   prometheus.Gauge
	avgSolidificationTime prometheus.Gauge
	messageMissingCountDB prometheus.Gauge
	messageRequestCount   prometheus.Gauge

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

	messageTotalCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_count_db",
		Help: "total number of messages in the node's database",
	})

	messageSolidCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_solid_count_db",
		Help: "number of solid messages on the node's database",
	})

	avgSolidificationTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_avg_solidification_time",
		Help: "average time it takes for a message to become solid",
	})

	messageMissingCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_missing_count_db",
		Help: "number of missing messages in the node's database",
	})

	transactionCounter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_transaction_counter",
		Help: "number of value transactions (value payloads) seen",
	})

	valueTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_value_tips",
		Help: "current number of tips in the value tangle",
	})

	messageRequestCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_request_queue_size",
		Help: "current number requested messages by the message tangle",
	})

	registry.MustRegister(messageTips)
	registry.MustRegister(messagePerTypeCount)
	registry.MustRegister(messageTotalCount)
	registry.MustRegister(messageTotalCountDB)
	registry.MustRegister(messageSolidCountDB)
	registry.MustRegister(avgSolidificationTime)
	registry.MustRegister(messageMissingCountDB)
	registry.MustRegister(messageRequestCount)
	registry.MustRegister(transactionCounter)
	registry.MustRegister(valueTips)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	messageTips.Set(float64(metrics.MessageTips()))
	msgCountPerPayload := metrics.MessageCountSinceStartPerPayload()
	for payloadType, count := range msgCountPerPayload {
		messagePerTypeCount.WithLabelValues(payload.Name(payloadType)).Set(float64(count))
	}
	messageTotalCount.Set(float64(metrics.MessageTotalCountSinceStart()))
	messageTotalCountDB.Set(float64(metrics.MessageTotalCountDB()))
	messageSolidCountDB.Set(float64(metrics.MessageSolidCountDB()))
	avgSolidificationTime.Set(metrics.AvgSolidificationTime())
	messageMissingCountDB.Set(float64(metrics.MessageMissingCountDB()))
	messageRequestCount.Set(float64(metrics.MessageRequestQueueSize()))
	transactionCounter.Set(float64(metrics.ValueTransactionCounter()))
	valueTips.Set(float64(metrics.ValueTips()))
}
