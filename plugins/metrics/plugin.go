package metrics

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	valuetangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
)

// PluginName is the name of the metrics plugin.
const PluginName = "Metrics"

var (
	// plugin is the plugin instance of the metrics plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {

	if config.Node().GetBool(CfgMetricsLocal) {
		registerLocalMetrics()
	}

	// Events from analysis server
	if config.Node().GetBool(CfgMetricsGlobal) {
		server.Events.MetricHeartbeat.Attach(onMetricHeartbeatReceived)
	}

	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(shutdownSignal <-chan struct{}) {
		if config.Node().GetBool(CfgMetricsLocal) {
			timeutil.Ticker(func() {
				measureCPUUsage()
				measureMemUsage()
				measureSynced()
				measureMessageTips()
				measureValueTips()
				measureReceivedMPS()
				measureRequestQueueSize()

				// gossip network traffic
				g := gossipCurrentTraffic()
				gossipCurrentRx.Store(uint64(g.BytesRead))
				gossipCurrentTx.Store(uint64(g.BytesWritten))
			}, 1*time.Second, shutdownSignal)
		}
		if config.Node().GetBool(CfgMetricsGlobal) {
			timeutil.Ticker(calculateNetworkDiameter, 1*time.Minute, shutdownSignal)
		}

	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerLocalMetrics() {
	//// Events declared in other packages which we want to listen to here ////

	// increase received MPS counter whenever we attached a message
	messagelayer.Tangle().Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		_payloadType := cachedMessage.Unwrap().Payload().Type()
		cachedMessage.Release()
		cachedMessageMetadata.Release()
		increaseReceivedMPSCounter()
		increasePerPayloadCounter(_payloadType)
	}))

	// Value payload attached
	valuetransfers.Tangle().Events.PayloadAttached.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *valuetangle.CachedPayloadMetadata) {
		cachedPayload.Release()
		cachedPayloadMetadata.Release()
		valueTransactionCounter.Inc()
	}))

	// FPC round executed
	valuetransfers.Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		processRoundStats(roundStats)
	}))

	// a conflict has been finalized
	valuetransfers.Voter().Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		processFinalized(ev.Ctx)
	}))

	// consensus failure in conflict resolution
	valuetransfers.Voter().Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		processFailed(ev.Ctx)
	}))

	//// Events coming from metrics package ////

	metrics.Events().FPCInboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		_FPCInboundBytes.Add(amountBytes)
	}))
	metrics.Events().FPCOutboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		_FPCOutboundBytes.Add(amountBytes)
	}))
	metrics.Events().AnalysisOutboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		analysisOutboundBytes.Add(amountBytes)
	}))
	metrics.Events().CPUUsage.Attach(events.NewClosure(func(cpuPercent float64) {
		cpuUsage.Store(cpuPercent)
	}))
	metrics.Events().MemUsage.Attach(events.NewClosure(func(memAllocBytes uint64) {
		memUsageBytes.Store(memAllocBytes)
	}))
	metrics.Events().Synced.Attach(events.NewClosure(func(synced bool) {
		isSynced.Store(synced)
	}))

	gossip.Manager().Events().NeighborRemoved.Attach(onNeighborRemoved)
	gossip.Manager().Events().NeighborAdded.Attach(onNeighborAdded)

	autopeering.Selection().Events().IncomingPeering.Attach(onAutopeeringSelection)
	autopeering.Selection().Events().OutgoingPeering.Attach(onAutopeeringSelection)

	metrics.Events().MessageTips.Attach(events.NewClosure(func(tipsCount uint64) {
		messageTips.Store(tipsCount)
	}))
	metrics.Events().ValueTips.Attach(events.NewClosure(func(tipsCount uint64) {
		valueTips.Store(tipsCount)
	}))

	metrics.Events().QueryReceived.Attach(events.NewClosure(func(ev *metrics.QueryReceivedEvent) {
		processQueryReceived(ev)
	}))
	metrics.Events().QueryReplyError.Attach(events.NewClosure(func(ev *metrics.QueryReplyErrorEvent) {
		processQueryReplyError(ev)
	}))
}
