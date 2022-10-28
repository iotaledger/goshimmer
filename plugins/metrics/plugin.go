package metrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/iotaledger/hive.go/core/timeutil"
	"github.com/iotaledger/hive.go/core/types"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"

	"github.com/iotaledger/goshimmer/packages/app/metrics"
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

	Protocol  *protocol.Protocol
	P2Pmgr    *p2p.Manager        `optional:"true"`
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
}

func registerLocalMetrics() {
	// // Events declared in other packages which we want to listen to here ////

	// increase received BPS counter whenever we attached a block
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		increaseReceivedBPSCounter()
		increasePerPayloadCounter(block.Payload().Type())

		sumTimesSinceIssued[Store] += time.Since(block.IssuingTime())
		increasePerComponentCounter(Store)
	}))

	// blocks can only become solid once, then they stay like that, hence no .Dec() part
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		increasePerComponentCounter(Solidifier)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// Consume should release cachedBlockMetadata
		if block.IsSolid() {
			// TODO: figure out whether to use retainer to get the times
			// sumTimesSinceReceived[Solidifier] += blkMetaData.ScheduledTime().Sub(blkMetaData.ReceivedTime())
			sumTimesSinceIssued[Solidifier] += time.Since(block.IssuingTime())
		}
	}))

	// fired when a block gets added to missing block storage
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(_ *blockdag.Block) {
		missingBlockCountDB.Inc()
		solidificationRequests.Inc()
	}))

	// fired when a missing block was received and removed from missing block storage
	deps.Protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(_ *blockdag.Block) {
		missingBlockCountDB.Dec()
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(Scheduler)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()
		schedulerTimeMutex.Lock()
		defer schedulerTimeMutex.Unlock()

		if block.IsScheduled() {
			// TODO: figure out whether to use retainer to get the times
			// sumSchedulerBookedTime += blkMetaData.ScheduledTime().Sub(blkMetaData.BookedTime())
			// sumTimesSinceReceived[Scheduler] += blkMetaData.ScheduledTime().Sub(blkMetaData.ReceivedTime())
			sumTimesSinceIssued[Scheduler] += time.Since(block.IssuingTime())
		}
	}))

	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		increasePerComponentCounter(Booker)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		if block.IsBooked() {
			// TODO: figure out whether to use retainer to get the times
			// sumTimesSinceReceived[Booker] += blkMetaData.BookedTime().Sub(blkMetaData.ReceivedTime())
			sumTimesSinceIssued[Booker] += time.Since(block.IssuingTime())
		}
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(SchedulerDropped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// TODO: figure out whether to use retainer to get the times
		// sumTimesSinceReceived[SchedulerDropped] += time.Since(blkMetaData.ReceivedTime())
		sumTimesSinceIssued[SchedulerDropped] += time.Since(block.IssuingTime())
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockSkipped.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(SchedulerSkipped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// TODO: figure out whether to use retainer to get the times
		// sumTimesSinceReceived[SchedulerSkipped] += time.Since(blkMetaData.ReceivedTime())
		sumTimesSinceIssued[SchedulerSkipped] += time.Since(block.IssuingTime())
	}))

	deps.Protocol.Events.Engine.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		blockType := DataBlock
		if block.Payload().Type() == devnetvm.TransactionType {
			blockType = Transaction
		}

		blockFinalizationTotalTimeMutex.Lock()
		defer blockFinalizationTotalTimeMutex.Unlock()
		finalizedBlockCountMutex.Lock()
		defer finalizedBlockCountMutex.Unlock()

		block.ForEachParent(func(parent models.Parent) {
			increasePerParentType(parent.Type)
		})
		blockFinalizationIssuedTotalTime[blockType] += uint64(time.Since(block.IssuingTime()).Milliseconds())
		// TODO: figure out whether to use retainer to get the times
		// blockFinalizationReceivedTotalTime[blockType] += uint64(time.Since(blockMetadata.ReceivedTime()).Milliseconds())
		finalizedBlockCount[blockType]++
	}))

	// TODO: add metrics for BlockUnorphaned count as well
	// fired when a message gets added to missing message storage
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(_ *blockdag.Block) {
		orphanedBlocks.Inc()
	}))

	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(event.NewClosure(func(conflictID utxo.TransactionID) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		if _, exists := activeConflicts[conflictID]; !exists {
			return
		}
		firstAttachment := deps.Protocol.Engine().Tangle.GetEarliestAttachment(conflictID)
		deps.Protocol.Engine().Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
			if _, exists := activeConflicts[conflictID]; exists && conflictingConflictID != conflictID {
				finalizedConflictCountDB.Inc()
				delete(activeConflicts, conflictingConflictID)
			}
			return true
		})
		finalizedConflictCountDB.Inc()
		confirmedConflictCount.Inc()
		conflictConfirmationTotalTime.Add(uint64(time.Since(firstAttachment.IssuingTime()).Milliseconds()))

		delete(activeConflicts, conflictID)
	}))

	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
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

	deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(onEpochCommitted)
}
