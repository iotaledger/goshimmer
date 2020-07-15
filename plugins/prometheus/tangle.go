package prometheus

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messageTips             prometheus.Gauge
	messagePerTypeCount     *prometheus.GaugeVec
	messageTotalCount       prometheus.Gauge
	messageTotalCountDBIter prometheus.Gauge
	messageTotalCountDBInc  prometheus.Gauge
	messageSolidCountDBIter prometheus.Gauge
	messageSolidCountDBInc  prometheus.Gauge
	avgSolidificationTime   prometheus.Gauge
	messageRequestCount     prometheus.Gauge

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

	messageTotalCountDBIter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_count_db_iter",
		Help: "total number of messages in the node's database",
	})

	messageTotalCountDBInc = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_count_db_inc",
		Help: "total number of messages in the node's database",
	})

	messageSolidCountDBIter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_solid_count_int",
		Help: "number of solid messages on the node's database",
	})

	messageSolidCountDBInc = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_solid_count_inc",
		Help: "number of solid messages on the node's database",
	})

	avgSolidificationTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_avg_solidification_time",
		Help: "average time it takes for a message to become solid",
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
	registry.MustRegister(messageTotalCountDBIter)
	registry.MustRegister(messageSolidCountDBIter)
	registry.MustRegister(messageTotalCountDBInc)
	registry.MustRegister(messageSolidCountDBInc)
	registry.MustRegister(avgSolidificationTime)
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
	messageTotalCountDBIter.Set(float64(metrics.MessageTotalCountDBIter()))
	messageSolidCountDBIter.Set(float64(metrics.MessageSolidCountIter()))
	messageTotalCountDBInc.Set(float64(metrics.MessageTotalCountDBInc()))
	messageSolidCountDBInc.Set(float64(metrics.MessageSolidCountInc()))
	avgSolidificationTime.Set(metrics.AvgSolidificationTime())
	messageRequestCount.Set(float64(metrics.MessageRequestQueueSize()))
	transactionCounter.Set(float64(metrics.ValueTransactionCounter()))
	valueTips.Set(float64(metrics.ValueTips()))
}
