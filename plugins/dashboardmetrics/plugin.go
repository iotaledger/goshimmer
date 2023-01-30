package dashboardmetrics

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

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
)

// PluginName is the name of the metrics plugin.
const PluginName = "DashboardMetrics"

var (
	// Plugin is the plugin instance of the metrics plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Protocol    *protocol.Protocol
	BlockIssuer *blockissuer.BlockIssuer

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
	registerLocalMetrics()
	measureInitialConflictStats()
	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(ctx context.Context) {
		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() {
			measureAttachedBPS()
			measureRequestQueueSize()
			measurePerComponentCounter()
		}, 1*time.Second, ctx)

		// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

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
	// increase received BPS counter whenever we attached a block
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		increaseReceivedBPSCounter()
		increasePerComponentCounter(collector.Attached)
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(collector.Scheduled)
	}))

	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		if block.IsBooked() {
			increasePerComponentCounter(collector.Booked)
			if block.Payload().Type() == devnetvm.TransactionType {
				increaseBookedTransactionCounter()
			}
		}
	}))

	deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		blockType := collector.DataBlock
		if block.Payload().Type() == devnetvm.TransactionType {
			blockType = collector.Transaction
		}

		increaseFinalizationIssuedTotalTime(blockType, uint64(time.Since(block.IssuingTime()).Milliseconds()))
		increaseFinalizedBlkPerTypeCounter(blockType)
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

}
