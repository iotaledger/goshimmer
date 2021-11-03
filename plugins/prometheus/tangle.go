package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	messageTips                  prometheus.Gauge
	messagePerTypeCount          *prometheus.GaugeVec
	messagePerComponentCount     *prometheus.GaugeVec
	parentsCount                 *prometheus.GaugeVec
	messageTotalCount            prometheus.Gauge
	messageTotalCountDB          prometheus.Gauge
	messageSolidCountDB          prometheus.Gauge
	solidificationTotalTime      prometheus.Gauge
	messageMissingCountDB        prometheus.Gauge
	messageRequestCount          prometheus.Gauge
	confirmedBranchCount         prometheus.Gauge
	branchConfirmationTotalTime  prometheus.Gauge
	totalBranchCountDB           prometheus.Gauge
	finalizedBranchCountDB       prometheus.Gauge
	finalizedMessageCount        *prometheus.GaugeVec
	messageFinalizationTotalTime *prometheus.GaugeVec
	transactionCounter           prometheus.Gauge
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

	parentsCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_parent_count_per_type",
			Help: "number of parents of all messages",
		}, []string{
			"type",
		})

	messagePerComponentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_per_component_count",
			Help: "number of messages per component seen since the start of the node",
		}, []string{
			"component",
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

	solidificationTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
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

	messageRequestCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_request_queue_size",
		Help: "current number requested messages by the message tangle",
	})

	messageFinalizationTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_finalization_time",
			Help: "total number of milliseconds taken for messages to finalize",
		}, []string{
			"messageType",
		})

	finalizedMessageCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_finalized_count",
			Help: "current number of finalized messages per type",
		}, []string{
			"messageType",
		})

	branchConfirmationTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_branch_confirmation_time",
		Help: "total number of milliseconds taken for branch to finalize",
	})

	totalBranchCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_branch_total_count_db",
		Help: "total number branches stored in database",
	})

	finalizedBranchCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_branch_finalized_count_db",
		Help: "number of finalized branches stored in database",
	})
	confirmedBranchCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_branch_confirmed_count",
		Help: "current number of confirmed branches",
	})

	registry.MustRegister(messageTips)
	registry.MustRegister(messagePerTypeCount)
	registry.MustRegister(parentsCount)
	registry.MustRegister(messagePerComponentCount)
	registry.MustRegister(messageTotalCount)
	registry.MustRegister(messageTotalCountDB)
	registry.MustRegister(messageSolidCountDB)
	registry.MustRegister(solidificationTotalTime)
	registry.MustRegister(messageMissingCountDB)
	registry.MustRegister(messageRequestCount)
	registry.MustRegister(messageFinalizationTotalTime)
	registry.MustRegister(finalizedMessageCount)
	registry.MustRegister(branchConfirmationTotalTime)
	registry.MustRegister(confirmedBranchCount)
	registry.MustRegister(totalBranchCountDB)
	registry.MustRegister(finalizedBranchCountDB)
	registry.MustRegister(transactionCounter)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	messageTips.Set(float64(metrics.MessageTips()))
	msgCountPerPayload := metrics.MessageCountSinceStartPerPayload()
	for payloadType, count := range msgCountPerPayload {
		messagePerTypeCount.WithLabelValues(payloadType.String()).Set(float64(count))
	}
	msgCountPerComponent := metrics.MessageCountSinceStartPerComponentGrafana()
	for component, count := range msgCountPerComponent {
		messagePerComponentCount.WithLabelValues(component.String()).Set(float64(count))
	}
	messageTotalCount.Set(float64(metrics.MessageTotalCountSinceStart()))
	messageTotalCountDB.Set(float64(metrics.MessageTotalCountDB()))
	messageSolidCountDB.Set(float64(metrics.MessageSolidCountDB()))
	solidificationTotalTime.Set(float64(metrics.SolidificationTime()))
	messageMissingCountDB.Set(float64(metrics.MessageMissingCountDB()))
	messageRequestCount.Set(float64(metrics.MessageRequestQueueSize()))
	confirmedBranchCount.Set(float64(metrics.ConfirmedBranchCount()))
	branchConfirmationTotalTime.Set(float64(metrics.BranchConfirmationTotalTime()))
	totalBranchCountDB.Set(float64(metrics.TotalBranchCountDB()))
	finalizedBranchCountDB.Set(float64(metrics.FinalizedBranchCountDB()))

	finalizedMessageCountPerType := metrics.FinalizedMessageCountPerType()
	for messageType, count := range finalizedMessageCountPerType {
		finalizedMessageCount.WithLabelValues(messageType.String()).Set(float64(count))
	}
	messageFinalizationTotalTimePerType := metrics.MessageFinalizationTotalTimePerType()
	for messageType, count := range messageFinalizationTotalTimePerType {
		messageFinalizationTotalTime.WithLabelValues(messageType.String()).Set(float64(count))
	}
	parentsCountPerType := metrics.ParentCountPerType()
	for parentType, count := range parentsCountPerType {
		parentsCount.WithLabelValues(parentType.String()).Set(float64(count))
	}
	// transactionCounter.Set(float64(metrics.ValueTransactionCounter()))
}
