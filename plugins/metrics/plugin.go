package metrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
	GossipMgr *gossip.Manager     `optional:"true"`
	Selection *selection.Protocol `optional:"true"`
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
		measureInitialBranchStats()
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
				measureSchedulerMetrics()
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
	// // Events declared in other packages which we want to listen to here ////

	// increase received MPS counter whenever we attached a message
	deps.Tangle.Storage.Events.MessageStored.Attach(event.NewClosure(func(event *tangle.MessageStoredEvent) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		messageID := event.MessageID
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			increaseReceivedMPSCounter()
			increasePerPayloadCounter(message.Payload().Type())

			deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
				sumTimesSinceIssued[Store] += msgMetaData.ReceivedTime().Sub(message.IssuingTime())
			})
		})
		increasePerComponentCounter(Store)
	}))

	// messages can only become solid once, then they stay like that, hence no .Dec() part
	deps.Tangle.Solidifier.Events.MessageSolid.Attach(event.NewClosure(func(event *tangle.MessageSolidEvent) {
		increasePerComponentCounter(Solidifier)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// Consume should release cachedMessageMetadata
		deps.Tangle.Storage.MessageMetadata(event.MessageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			if msgMetaData.IsSolid() {
				sumTimesSinceReceived[Solidifier] += msgMetaData.SolidificationTime().Sub(msgMetaData.ReceivedTime())
			}
		})
	}))

	// fired when a message gets added to missing message storage
	deps.Tangle.Solidifier.Events.MessageMissing.Attach(event.NewClosure(func(_ *tangle.MessageMissingEvent) {
		missingMessageCountDB.Inc()
		solidificationRequests.Inc()
	}))

	// fired when a missing message was received and removed from missing message storage
	deps.Tangle.Storage.Events.MissingMessageStored.Attach(event.NewClosure(func(_ *tangle.MissingMessageStoredEvent) {
		missingMessageCountDB.Dec()
	}))

	deps.Tangle.Scheduler.Events.MessageScheduled.Attach(event.NewClosure(func(event *tangle.MessageScheduledEvent) {
		increasePerComponentCounter(Scheduler)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		schedulerTimeMutex.Lock()
		defer schedulerTimeMutex.Unlock()

		messageID := event.MessageID
		// Consume should release cachedMessageMetadata
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			if msgMetaData.Scheduled() {
				sumSchedulerBookedTime += msgMetaData.ScheduledTime().Sub(msgMetaData.BookedTime())

				sumTimesSinceReceived[Scheduler] += msgMetaData.ScheduledTime().Sub(msgMetaData.ReceivedTime())
				deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
					sumTimesSinceIssued[Scheduler] += msgMetaData.ScheduledTime().Sub(message.IssuingTime())
				})
			}
		})
	}))

	deps.Tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *tangle.MessageBookedEvent) {
		increasePerComponentCounter(Booker)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		messageID := event.MessageID
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			if msgMetaData.IsBooked() {
				sumTimesSinceReceived[Booker] += msgMetaData.BookedTime().Sub(msgMetaData.ReceivedTime())
				deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
					sumTimesSinceIssued[Booker] += msgMetaData.BookedTime().Sub(message.IssuingTime())
				})
			}
		})
	}))

	deps.Tangle.Scheduler.Events.MessageDiscarded.Attach(event.NewClosure(func(event *tangle.MessageDiscardedEvent) {
		increasePerComponentCounter(SchedulerDropped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		messageID := event.MessageID
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			sumTimesSinceReceived[SchedulerDropped] += clock.Since(msgMetaData.ReceivedTime())
			deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
				sumTimesSinceIssued[SchedulerDropped] += clock.Since(message.IssuingTime())
			})
		})
	}))

	deps.Tangle.Scheduler.Events.MessageSkipped.Attach(event.NewClosure(func(event *tangle.MessageSkippedEvent) {
		increasePerComponentCounter(SchedulerSkipped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		messageID := event.MessageID
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(msgMetaData *tangle.MessageMetadata) {
			sumTimesSinceReceived[SchedulerSkipped] += clock.Since(msgMetaData.ReceivedTime())
			deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
				sumTimesSinceIssued[SchedulerSkipped] += clock.Since(message.IssuingTime())
			})
		})
	}))

	deps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		messageType := DataMessage
		messageID := event.MessageID
		deps.Tangle.Utils.ComputeIfTransaction(messageID, func(_ utxo.TransactionID) {
			messageType = Transaction
		})
		messageFinalizationTotalTimeMutex.Lock()
		defer messageFinalizationTotalTimeMutex.Unlock()
		finalizedMessageCountMutex.Lock()
		defer finalizedMessageCountMutex.Unlock()

		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			message.ForEachParent(func(parent tangle.Parent) {
				increasePerParentType(parent.Type)
			})
			messageFinalizationIssuedTotalTime[messageType] += uint64(clock.Since(message.IssuingTime()).Milliseconds())
		})
		if deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			messageFinalizationReceivedTotalTime[messageType] += uint64(clock.Since(messageMetadata.ReceivedTime()).Milliseconds())
		}) {
			finalizedMessageCount[messageType]++
		}
	}))

	deps.Tangle.Ledger.BranchDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *branchdag.BranchConfirmedEvent) {
		activeBranchesMutex.Lock()
		defer activeBranchesMutex.Unlock()

		branchID := event.BranchID
		if _, exists := activeBranches[branchID]; !exists {
			return
		}
		oldestAttachmentTime, _, err := deps.Tangle.Utils.FirstAttachment(utxo.TransactionID(branchID))
		if err != nil {
			return
		}
		deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID branchdag.BranchID) bool {
			if _, exists := activeBranches[branchID]; exists && conflictingBranchID != branchID {
				finalizedBranchCountDB.Inc()
				delete(activeBranches, conflictingBranchID)
			}
			return true
		})
		finalizedBranchCountDB.Inc()
		confirmedBranchCount.Inc()
		branchConfirmationTotalTime.Add(uint64(clock.Since(oldestAttachmentTime).Milliseconds()))

		delete(activeBranches, branchID)
	}))

	deps.Tangle.Ledger.BranchDAG.Events.BranchCreated.Attach(event.NewClosure(func(event *branchdag.BranchCreatedEvent) {
		activeBranchesMutex.Lock()
		defer activeBranchesMutex.Unlock()

		branchID := event.BranchID
		if _, exists := activeBranches[branchID]; !exists {
			branchTotalCountDB.Inc()
			activeBranches[branchID] = types.Void
		}
	}))

	metrics.Events.AnalysisOutboundBytes.Attach(event.NewClosure(func(event *metrics.AnalysisOutboundBytesEvent) {
		analysisOutboundBytes.Add(event.AmountBytes)
	}))
	metrics.Events.CPUUsage.Attach(event.NewClosure(func(evnet *metrics.CPUUsageEvent) {
		cpuUsage.Store(evnet.CPUPercent)
	}))
	metrics.Events.MemUsage.Attach(event.NewClosure(func(event *metrics.MemUsageEvent) {
		memUsageBytes.Store(event.MemAllocBytes)
	}))

	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(onNeighborRemoved)
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(onNeighborAdded)

	if deps.Selection != nil {
		deps.Selection.Events().IncomingPeering.Attach(onAutopeeringSelection)
		deps.Selection.Events().OutgoingPeering.Attach(onAutopeeringSelection)
	}

	// mana pledge events
	mana.Events.Pledged.Attach(event.NewClosure(func(ev *mana.PledgedEvent) {
		addPledge(ev)
	}))
}
