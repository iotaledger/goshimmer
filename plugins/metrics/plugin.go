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
	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/p2p"
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

	Tangle          *tangle.Tangle
	P2Pmgr          *p2p.Manager        `optional:"true"`
	Selection       *selection.Protocol `optional:"true"`
	Local           *peer.Local
	NotarizationMgr *notarization.Manager
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
		// initial measurement, since we have to know how many blocks are there in the db
		measureInitialDBStats()
		measureInitialConflictStats()
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
				measureBlockTips()
				measureReceivedBPS()
				measureRequestQueueSize()
				measureGossipTraffic()
				measurePerComponentCounter()
				measureRateSetter()
				measureSchedulerMetrics()
			}, 1*time.Second, ctx)
		}

		if Parameters.Global {
			// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
			// safely ignore the last execution when shutting down.
			timeutil.NewTicker(calculateNetworkDiameter, 1*time.Minute, ctx)
		}

		// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	// create a background worker that updates the mana metrics
	if err := daemon.BackgroundWorker("Metrics Mana Updater", func(ctx context.Context) {
		if deps.P2Pmgr == nil {
			return
		}
		defer log.Infof("Stopping Metrics Mana Updater ... done")
		timeutil.NewTicker(func() {
			measureMana()
		}, Parameters.ManaUpdateInterval, ctx)
		// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
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
			// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
			<-ctx.Done()
			log.Infof("Stopping Metrics Research Mana Updater ...")
		}, shutdown.PriorityMetrics); err != nil {
			log.Panicf("Failed to start as daemon: %s", err)
		}
	}
}

func registerLocalMetrics() {
	// // Events declared in other packages which we want to listen to here ////

	// increase received BPS counter whenever we attached a block
	deps.Tangle.Storage.Events.BlockStored.Attach(event.NewClosure(func(event *tangle.BlockStoredEvent) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		increaseReceivedBPSCounter()
		increasePerPayloadCounter(event.Block.Payload().Type())

		deps.Tangle.Storage.BlockMetadata(event.Block.ID()).Consume(func(blkMetaData *tangle.BlockMetadata) {
			sumTimesSinceIssued[Store] += blkMetaData.ReceivedTime().Sub(event.Block.IssuingTime())
		})
		increasePerComponentCounter(Store)
	}))

	// blocks can only become solid once, then they stay like that, hence no .Dec() part
	deps.Tangle.Solidifier.Events.BlockSolid.Attach(event.NewClosure(func(event *tangle.BlockSolidEvent) {
		increasePerComponentCounter(Solidifier)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// Consume should release cachedBlockMetadata
		deps.Tangle.Storage.BlockMetadata(event.Block.ID()).Consume(func(blkMetaData *tangle.BlockMetadata) {
			if blkMetaData.IsSolid() {
				sumTimesSinceReceived[Solidifier] += blkMetaData.SolidificationTime().Sub(blkMetaData.ReceivedTime())
			}
		})
	}))

	// fired when a block gets added to missing block storage
	deps.Tangle.Solidifier.Events.BlockMissing.Attach(event.NewClosure(func(_ *tangle.BlockMissingEvent) {
		missingBlockCountDB.Inc()
		solidificationRequests.Inc()
	}))

	// fired when a missing block was received and removed from missing block storage
	deps.Tangle.Storage.Events.MissingBlockStored.Attach(event.NewClosure(func(_ *tangle.MissingBlockStoredEvent) {
		missingBlockCountDB.Dec()
	}))

	deps.Tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *tangle.BlockScheduledEvent) {
		increasePerComponentCounter(Scheduler)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		schedulerTimeMutex.Lock()
		defer schedulerTimeMutex.Unlock()

		blockID := event.BlockID
		// Consume should release cachedBlockMetadata
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetaData *tangle.BlockMetadata) {
			if blkMetaData.Scheduled() {
				sumSchedulerBookedTime += blkMetaData.ScheduledTime().Sub(blkMetaData.BookedTime())

				sumTimesSinceReceived[Scheduler] += blkMetaData.ScheduledTime().Sub(blkMetaData.ReceivedTime())
				deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
					sumTimesSinceIssued[Scheduler] += blkMetaData.ScheduledTime().Sub(block.IssuingTime())
				})
			}
		})
	}))

	deps.Tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *tangle.BlockBookedEvent) {
		increasePerComponentCounter(Booker)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		blockID := event.BlockID
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetaData *tangle.BlockMetadata) {
			if blkMetaData.IsBooked() {
				sumTimesSinceReceived[Booker] += blkMetaData.BookedTime().Sub(blkMetaData.ReceivedTime())
				deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
					sumTimesSinceIssued[Booker] += blkMetaData.BookedTime().Sub(block.IssuingTime())
				})
			}
		})
	}))

	deps.Tangle.Scheduler.Events.BlockDiscarded.Attach(event.NewClosure(func(event *tangle.BlockDiscardedEvent) {
		increasePerComponentCounter(SchedulerDropped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		blockID := event.BlockID
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetaData *tangle.BlockMetadata) {
			sumTimesSinceReceived[SchedulerDropped] += clock.Since(blkMetaData.ReceivedTime())
			deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
				sumTimesSinceIssued[SchedulerDropped] += clock.Since(block.IssuingTime())
			})
		})
	}))

	deps.Tangle.Scheduler.Events.BlockSkipped.Attach(event.NewClosure(func(event *tangle.BlockSkippedEvent) {
		increasePerComponentCounter(SchedulerSkipped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		blockID := event.BlockID
		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blkMetaData *tangle.BlockMetadata) {
			sumTimesSinceReceived[SchedulerSkipped] += clock.Since(blkMetaData.ReceivedTime())
			deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
				sumTimesSinceIssued[SchedulerSkipped] += clock.Since(block.IssuingTime())
			})
		})
	}))

	deps.Tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *tangle.BlockAcceptedEvent) {
		blockType := DataBlock
		block := event.Block
		blockID := block.ID()
		deps.Tangle.Utils.ComputeIfTransaction(blockID, func(_ utxo.TransactionID) {
			blockType = Transaction
		})
		blockFinalizationTotalTimeMutex.Lock()
		defer blockFinalizationTotalTimeMutex.Unlock()
		finalizedBlockCountMutex.Lock()
		defer finalizedBlockCountMutex.Unlock()

		block.ForEachParent(func(parent tangle.Parent) {
			increasePerParentType(parent.Type)
		})
		blockFinalizationIssuedTotalTime[blockType] += uint64(clock.Since(block.IssuingTime()).Milliseconds())
		if deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangle.BlockMetadata) {
			blockFinalizationReceivedTotalTime[blockType] += uint64(clock.Since(blockMetadata.ReceivedTime()).Milliseconds())
		}) {
			finalizedBlockCount[blockType]++
		}
	}))

	// fired when a message gets added to missing message storage
	deps.Tangle.OrphanageManager.Events.BlockOrphaned.Attach(event.NewClosure(func(evt *tangle.BlockOrphanedEvent) {
		orphanedBlocks.Inc()
	}))

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		conflictID := event.ID
		if _, exists := activeConflicts[conflictID]; !exists {
			return
		}
		oldestAttachmentTime, _, err := deps.Tangle.Utils.FirstAttachment(conflictID)
		if err != nil {
			return
		}
		deps.Tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
			if _, exists := activeConflicts[conflictID]; exists && conflictingConflictID != conflictID {
				finalizedConflictCountDB.Inc()
				delete(activeConflicts, conflictingConflictID)
			}
			return true
		})
		finalizedConflictCountDB.Inc()
		confirmedConflictCount.Inc()
		conflictConfirmationTotalTime.Add(uint64(clock.Since(oldestAttachmentTime).Milliseconds()))

		delete(activeConflicts, conflictID)
	}))

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		conflictID := event.ID
		if _, exists := activeConflicts[conflictID]; !exists {
			conflictTotalCountDB.Inc()
			activeConflicts[conflictID] = types.Void
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

	deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(onNeighborRemoved)
	deps.P2Pmgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Attach(onNeighborAdded)

	if deps.Selection != nil {
		deps.Selection.Events().IncomingPeering.Hook(onAutopeeringSelection)
		deps.Selection.Events().OutgoingPeering.Hook(onAutopeeringSelection)
	}

	// mana pledge events
	mana.Events.Pledged.Attach(event.NewClosure(func(ev *mana.PledgedEvent) {
		addPledge(ev)
	}))

	deps.NotarizationMgr.Events.EpochCommittable.Attach(onEpochCommitted)
}
