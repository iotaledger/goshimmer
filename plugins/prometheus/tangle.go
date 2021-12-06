package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	messageTips                               prometheus.Gauge
	solidificationRequests                    prometheus.Gauge
	messagePerTypeCount                       *prometheus.GaugeVec
	initialMessagePerComponentCount           *prometheus.GaugeVec
	messagePerComponentCount                  *prometheus.GaugeVec
	initialSinceReceivedTotalTime             *prometheus.GaugeVec
	sinceReceivedTotalTime                    *prometheus.GaugeVec
	sinceIssuedTotalTime                      *prometheus.GaugeVec
	initialSchedulerTotalTime                 prometheus.Gauge
	schedulerTotalTime                        prometheus.Gauge
	parentsCount                              *prometheus.GaugeVec
	initialMissingMessagesCountDB             prometheus.Gauge
	messageMissingCountDB                     prometheus.Gauge
	messageRequestCount                       prometheus.Gauge
	confirmedBranchCount                      prometheus.Gauge
	branchConfirmationTotalTime               prometheus.Gauge
	totalBranchCountDB                        prometheus.Gauge
	finalizedBranchCountDB                    prometheus.Gauge
	finalizedMessageCount                     *prometheus.GaugeVec
	messageFinalizationTotalTimeSinceReceived *prometheus.GaugeVec
	messageFinalizationTotalTimeSinceIssued   *prometheus.GaugeVec
)

func registerTangleMetrics() {
	messageTips = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_tips_count",
		Help: "Current number of tips in message tangle",
	})

	solidificationRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_solidification_missing_message_count",
		Help: "Total number of messages requested by Solidifier.",
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
	initialMessagePerComponentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_initial_messages_per_component_count",
			Help: "number of messages per component seen since at the start of the node",
		}, []string{
			"component",
		})

	messagePerComponentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_messages_per_component_count",
			Help: "number of messages per component seen since the start of the node",
		}, []string{
			"component",
		})

	initialSinceReceivedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_initial_message_total_time_since_received",
			Help: "total time it took for a message to be processed by each component since it has been received at the start of the node",
		}, []string{
			"component",
		})

	sinceReceivedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_total_time_since_received",
			Help: "total time it took for a message to be processed by each component since it has been received, since the start of the node",
		}, []string{
			"component",
		})

	sinceIssuedTotalTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_total_time_since_issued",
			Help: "total time it took for a message to be processed by each component since it has been issued since the start of the node",
		}, []string{
			"component",
		})

	initialSchedulerTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_initial_total_scheduling_time",
		Help: "total time the scheduled messages spend in the scheduling queue at the start of the node",
	})

	schedulerTotalTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_total_scheduling_time",
		Help: "total time the scheduled messages spend in the scheduling queue since the node start",
	})

	messageMissingCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_missing_initial_count_db",
		Help: "number of missing messages in the node's database at the start of the node",
	})

	initialMissingMessagesCountDB = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_missing_count_db",
		Help: "number of missing messages in the node's database since the start of the node",
	})

	messageRequestCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tangle_message_request_queue_size",
		Help: "current number requested messages by the message tangle",
	})

	messageFinalizationTotalTimeSinceReceived = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_finalization_time_since_received",
			Help: "total number of milliseconds taken for messages to finalize since message received",
		}, []string{
			"messageType",
		})
	messageFinalizationTotalTimeSinceIssued = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tangle_message_finalization_time_since_issued",
			Help: "total number of milliseconds taken for messages to finalize since message issued",
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
	registry.MustRegister(solidificationRequests)
	registry.MustRegister(messagePerTypeCount)
	registry.MustRegister(parentsCount)
	registry.MustRegister(initialMessagePerComponentCount)
	registry.MustRegister(messagePerComponentCount)
	registry.MustRegister(sinceIssuedTotalTime)
	registry.MustRegister(initialSinceReceivedTotalTime)
	registry.MustRegister(sinceReceivedTotalTime)
	registry.MustRegister(initialSchedulerTotalTime)
	registry.MustRegister(schedulerTotalTime)
	registry.MustRegister(initialMissingMessagesCountDB)
	registry.MustRegister(messageMissingCountDB)
	registry.MustRegister(messageRequestCount)
	registry.MustRegister(messageFinalizationTotalTimeSinceReceived)
	registry.MustRegister(messageFinalizationTotalTimeSinceIssued)
	registry.MustRegister(finalizedMessageCount)
	registry.MustRegister(branchConfirmationTotalTime)
	registry.MustRegister(confirmedBranchCount)
	registry.MustRegister(totalBranchCountDB)
	registry.MustRegister(finalizedBranchCountDB)

	addCollect(collectTangleMetrics)
}

func collectTangleMetrics() {
	messageTips.Set(float64(metrics.MessageTips()))
	solidificationRequests.Set(float64(metrics.SolidificationRequests()))
	msgCountPerPayload := metrics.MessageCountSinceStartPerPayload()
	for payloadType, count := range msgCountPerPayload {
		messagePerTypeCount.WithLabelValues(payloadType.String()).Set(float64(count))
	}

	initialMsgCountPerComponent := metrics.InitialMessageCountPerComponentGrafana()
	for component, count := range initialMsgCountPerComponent {
		initialMessagePerComponentCount.WithLabelValues(component.String()).Set(float64(count))
	}

	msgCountPerComponent := metrics.MessageCountSinceStartPerComponentGrafana()
	for component, count := range msgCountPerComponent {
		messagePerComponentCount.WithLabelValues(component.String()).Set(float64(count))
	}

	initialSumTimeSinceReceived := metrics.InitialSumTimeSinceReceived()
	for component, count := range initialSumTimeSinceReceived {
		initialSinceReceivedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}

	sumTimeSinceReceived := metrics.SumTimeSinceReceived()
	for component, count := range sumTimeSinceReceived {
		sinceReceivedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}
	sumTimeSinceIssued := metrics.SumTimeSinceIssued()
	for component, count := range sumTimeSinceIssued {
		sinceIssuedTotalTime.WithLabelValues(component.String()).Set(float64(count))
	}

	initialSchedulerTotalTime.Set(float64(metrics.InitialSchedulerTime()))
	schedulerTotalTime.Set(float64(metrics.SchedulerTime()))
	initialMissingMessagesCountDB.Set(float64(metrics.InitialMessageMissingCountDB()))
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
	messageFinalizationTotalTimeSinceReceivePerType := metrics.MessageFinalizationTotalTimeSinceReceivedPerType()
	for messageType, count := range messageFinalizationTotalTimeSinceReceivePerType {
		messageFinalizationTotalTimeSinceReceived.WithLabelValues(messageType.String()).Set(float64(count))
	}
	messageFinalizationTotalTimeSinceIssuederType := metrics.MessageFinalizationTotalTimeSinceIssuedPerType()
	for messageType, count := range messageFinalizationTotalTimeSinceIssuederType {
		messageFinalizationTotalTimeSinceIssued.WithLabelValues(messageType.String()).Set(float64(count))
	}

	parentsCountPerType := metrics.ParentCountPerType()
	for parentType, count := range parentsCountPerType {
		parentsCount.WithLabelValues(parentType.String()).Set(float64(count))
	}
}
