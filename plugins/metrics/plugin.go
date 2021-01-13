package metrics

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	valuetangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/consensus"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/objectstorage"
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
	log.Infof("Starting %s ...", PluginName)
	if config.Node().Bool(CfgMetricsLocal) {
		// initial measurement, since we have to know how many messages are there in the db
		measureInitialDBStats()
		registerLocalMetrics()
	}

	// Events from analysis server
	if config.Node().Bool(CfgMetricsGlobal) {
		server.Events.MetricHeartbeat.Attach(onMetricHeartbeatReceived)
	}

	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(shutdownSignal <-chan struct{}) {
		if config.Node().Bool(CfgMetricsLocal) {
			// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
			// safely ignore the last execution when shutting down.
			timeutil.NewTicker(func() {
				measureCPUUsage()
				measureMemUsage()
				measureSynced()
				measureMessageTips()
				measureValueTips()
				measureReceivedMPS()
				measureRequestQueueSize()
				measureGossipTraffic()
			}, 1*time.Second, shutdownSignal)
		}

		if config.Node().Bool(CfgMetricsGlobal) {
			// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
			// safely ignore the last execution when shutting down.
			timeutil.NewTicker(calculateNetworkDiameter, 1*time.Minute, shutdownSignal)
		}

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-shutdownSignal
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerLocalMetrics() {
	//// Events declared in other packages which we want to listen to here ////

	// increase received MPS counter whenever we attached a message
	messagelayer.Tangle().Events.MessageAttached.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		_payloadType := cachedMsgEvent.Message.Unwrap().Payload().Type()
		cachedMsgEvent.Message.Release()
		cachedMsgEvent.MessageMetadata.Release()
		increaseReceivedMPSCounter()
		increasePerPayloadCounter(_payloadType)
		// MessageAttached is triggered in storeMessageWorker that saves the msg to database
		messageTotalCountDB.Inc()

	}))

	messagelayer.Tangle().Events.MessageRemoved.Attach(events.NewClosure(func(messageId tangle.MessageID) {
		// MessageRemoved triggered when the message gets removed from database.
		messageTotalCountDB.Dec()
	}))

	// messages can only become solid once, then they stay like that, hence no .Dec() part
	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.Message.Release()
		solidTimeMutex.Lock()
		defer solidTimeMutex.Unlock()
		// Consume should release cachedMessageMetadata
		cachedMsgEvent.MessageMetadata.Consume(func(object objectstorage.StorableObject) {
			msgMetaData := object.(*tangle.MessageMetadata)
			if msgMetaData.IsSolid() {
				messageSolidCountDBInc.Inc()
				sumSolidificationTime += msgMetaData.SolidificationTime().Sub(msgMetaData.ReceivedTime())
			}
		})
	}))

	// fired when a message gets added to missing message storage
	messagelayer.Tangle().Events.MessageMissing.Attach(events.NewClosure(func(messageId tangle.MessageID) {
		missingMessageCountDB.Inc()
	}))

	// fired when a missing message was received and removed from missing message storage
	messagelayer.Tangle().Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		cachedMsgEvent.Message.Release()
		cachedMsgEvent.MessageMetadata.Release()
		missingMessageCountDB.Dec()
	}))

	// Value payload attached
	valuetransfers.Tangle().Events.PayloadAttached.Attach(events.NewClosure(func(cachedPayloadEvent *valuetangle.CachedPayloadEvent) {
		cachedPayloadEvent.Payload.Release()
		cachedPayloadEvent.PayloadMetadata.Release()
		valueTransactionCounter.Inc()
	}))

	// FPC round executed
	consensus.Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
		processRoundStats(roundStats)
	}))

	// a conflict has been finalized
	consensus.Voter().Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		processFinalized(ev.Ctx)
	}))

	// consensus failure in conflict resolution
	consensus.Voter().Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
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
