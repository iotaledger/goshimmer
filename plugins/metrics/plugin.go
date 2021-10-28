package metrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/server"
)

// PluginName is the name of the metrics plugin.
const PluginName = "Metrics"

var (
	// Plugin is the plugin instance of the metrics plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Tangle    *tangle.Tangle
	GossipMgr *gossip.Manager          `optional:"true"`
	Selection *selection.Protocol      `optional:"true"`
	Voter     vote.DRNGRoundBasedVoter `optional:"true"`
	Local     *peer.Local
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if Parameters.Local {
		// initial measurement, since we have to know how many messages are there in the db
		measureInitialDBStats()
		registerLocalMetrics()
	}
	// Events from analysis server
	if Parameters.Global {
		server.Events.MetricHeartbeat.Attach(onMetricHeartbeatReceived)
	}

	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(ctx context.Context) {
		if Parameters.Local {
			// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
			// safely ignore the last execution when shutting down.
			timeutil.NewTicker(func() {
				measureCPUUsage()
				measureMemUsage()
				measureSynced()
				measureMessageTips()
				measureReceivedMPS()
				measureRequestQueueSize()
				measureGossipTraffic()
				measurePerComponentCounter()
			}, 1*time.Second, ctx)
		}

		if Parameters.Global {
			// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
			// safely ignore the last execution when shutting down.
			timeutil.NewTicker(calculateNetworkDiameter, 1*time.Minute, ctx)
		}

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	// create a background worker that updates the mana metrics
	if err := daemon.BackgroundWorker("Metrics Mana Updater", func(ctx context.Context) {
		if deps.GossipMgr == nil {
			return
		}
		defer log.Infof("Stopping Metrics Mana Updater ... done")
		timeutil.NewTicker(func() {
			measureMana()
		}, Parameters.ManaUpdateInterval, ctx)
		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-ctx.Done()
		log.Infof("Stopping Metrics Mana Updater ...")
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	if Parameters.ManaResearch {
		// create a background worker that updates the research mana metrics
		if err := daemon.BackgroundWorker("Metrics Research Mana Updater", func(ctx context.Context) {
			defer log.Infof("Stopping Metrics Research Mana Updater ... done")
			timeutil.NewTicker(func() {
				measureAccessResearchMana()
				measureConsensusResearchMana()
			}, Parameters.ManaUpdateInterval, ctx)
			// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
			<-ctx.Done()
			log.Infof("Stopping Metrics Research Mana Updater ...")
		}, shutdown.PriorityMetrics); err != nil {
			log.Panicf("Failed to start as daemon: %s", err)
		}
	}
}

func registerLocalMetrics() {
	//// Events declared in other packages which we want to listen to here ////

	// increase received MPS counter whenever we attached a message
	deps.Tangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			increaseReceivedMPSCounter()
			increasePerPayloadCounter(message.Payload().Type())
			// MessageStored is triggered in storeMessageWorker that saves the msg to database
			messageTotalCountDB.Inc()
		})
		increasePerComponentCounter(Store)
	}))

	deps.Tangle.Storage.Events.MessageRemoved.Attach(events.NewClosure(func(messageId tangle.MessageID) {
		// MessageRemoved triggered when the message gets removed from database.
		messageTotalCountDB.Dec()
	}))

	// messages can only become solid once, then they stay like that, hence no .Dec() part
	deps.Tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		increasePerComponentCounter(Solidifier)
		solidTimeMutex.Lock()
		defer solidTimeMutex.Unlock()

		// Consume should release cachedMessageMetadata
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			if msgMetaData.IsSolid() {
				messageSolidCountDBInc.Inc()
				sumSolidificationTime += msgMetaData.SolidificationTime().Sub(msgMetaData.ReceivedTime())
			}
		})
	}))

	// fired when a message gets added to missing message storage
	deps.Tangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageId tangle.MessageID) {
		missingMessageCountDB.Inc()
	}))

	// fired when a missing message was received and removed from missing message storage
	deps.Tangle.Storage.Events.MissingMessageStored.Attach(events.NewClosure(func(tangle.MessageID) {
		missingMessageCountDB.Dec()
	}))

	deps.Tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		increasePerComponentCounter(Scheduler)
	}))

	deps.Tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(message tangle.MessageID) {
		increasePerComponentCounter(Booker)
	}))

	// // Value payload attached
	// valuetransfers.Tangle().Events.PayloadAttached.Attach(events.NewClosure(func(cachedPayloadEvent *valuetangle.CachedPayloadEvent) {
	// 	cachedPayloadEvent.Payload.Release()
	// 	cachedPayloadEvent.PayloadMetadata.Release()
	// 	valueTransactionCounter.Inc()
	// }))

	if deps.Voter != nil {
		// FPC round executed
		deps.Voter.Events().RoundExecuted.Attach(events.NewClosure(func(roundStats *vote.RoundStats) {
			processRoundStats(roundStats)
		}))

		// a conflict has been finalized
		deps.Voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			processFinalized(ev.Ctx)
		}))

		// consensus failure in conflict resolution
		deps.Voter.Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			processFailed(ev.Ctx)
		}))
	}

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
	metrics.Events().TangleTimeSynced.Attach(events.NewClosure(func(synced bool) {
		isTangleTimeSynced.Store(synced)
	}))

	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(onNeighborRemoved)
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(onNeighborAdded)

	if deps.Selection != nil {
		deps.Selection.Events().IncomingPeering.Attach(onAutopeeringSelection)
		deps.Selection.Events().OutgoingPeering.Attach(onAutopeeringSelection)
	}

	metrics.Events().MessageTips.Attach(events.NewClosure(func(tipsCount uint64) {
		messageTips.Store(tipsCount)
	}))

	metrics.Events().QueryReceived.Attach(events.NewClosure(func(ev *metrics.QueryReceivedEvent) {
		processQueryReceived(ev)
	}))
	metrics.Events().QueryReplyError.Attach(events.NewClosure(func(ev *metrics.QueryReplyErrorEvent) {
		processQueryReplyError(ev)
	}))

	// mana pledge events
	mana.Events().Pledged.Attach(events.NewClosure(func(ev *mana.PledgedEvent) {
		addPledge(ev)
	}))
}
