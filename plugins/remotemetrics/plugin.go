// Package remotemetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotemetrics

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/timeutil"
)

const (
	syncUpdateTime           = 500 * time.Millisecond
	schedulerQueryUpdateTime = 5 * time.Second
)

const (
	// PluginName is the name of the faucet plugin.
	PluginName = "RemoteLogMetrics"
)

const (
	// Debug defines the most verbose metrics collection level.
	Debug uint8 = iota
	// Info defines regular metrics collection level.
	Info
	// Important defines the level of collection of only most important metrics.
	Important
	// Critical defines the level of collection of only critical metrics.
	Critical
)

var (
	// Plugin is the plugin instance of the remote plugin instance.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Local        *peer.Local
	Protocol     *protocol.Protocol
	RemoteLogger *remotelog.RemoteLoggerConn `optional:"true"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	// if remotelog plugin is disabled, then remotemetrics should not be started either
	if node.IsSkipped(remotelog.Plugin) {
		Plugin.LogInfof("%s is disabled; skipping %s\n", remotelog.Plugin.Name, Plugin.Name)
		return
	}

	configureSyncMetrics(plugin)
	configureConflictConfirmationMetrics(plugin)
	configureBlockFinalizedMetrics(plugin)
	configureBlockScheduledMetrics(plugin)
	configureMissingBlockMetrics(plugin)
	configureSchedulerQueryMetrics(plugin)
}

func run(plugin *node.Plugin) {
	// if remotelog plugin is disabled, then remotemetrics should not be started either
	if node.IsSkipped(remotelog.Plugin) {
		return
	}

	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Node State Logger Updater", func(ctx context.Context) {
		measureInitialConflictCounts()

		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() { checkSynced() }, syncUpdateTime, ctx)
		timeutil.NewTicker(func() {
			remotemetrics.Events.SchedulerQuery.Trigger(&remotemetrics.SchedulerQueryEvent{Time: time.Now()})
		}, schedulerQueryUpdateTime, ctx)

		// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityRemoteLog); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureSyncMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	remotemetrics.Events.TangleTimeSyncChanged.Hook(func(event *remotemetrics.TangleTimeSyncChangedEvent) {
		isTangleTimeSynced.Store(event.CurrentStatus)
	}, event.WithWorkerPool(plugin.WorkerPool))
	remotemetrics.Events.TangleTimeSyncChanged.Hook(func(event *remotemetrics.TangleTimeSyncChangedEvent) {
		sendSyncStatusChangedEvent(event)
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func configureSchedulerQueryMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	remotemetrics.Events.SchedulerQuery.Hook(func(event *remotemetrics.SchedulerQueryEvent) { obtainSchedulerStats(event.Time) }, event.WithWorkerPool(plugin.WorkerPool))
}

func configureConflictConfirmationMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		onConflictConfirmed(conflict.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictCreated.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		if !activeConflicts.Has(conflict.ID()) {
			conflictTotalCountDB.Inc()
			activeConflicts.Add(conflict.ID())
			sendConflictMetrics()
		}
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func configureBlockFinalizedMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	if Parameters.MetricsLevel == Info {
		deps.Protocol.Events.Engine.Ledger.MemPool.TransactionAccepted.Hook(onTransactionAccepted, event.WithWorkerPool(plugin.WorkerPool))
	} else {
		deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
			onBlockFinalized(block.ModelsBlock)
		}, event.WithWorkerPool(plugin.WorkerPool))
	}
}

func configureBlockScheduledMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	if Parameters.MetricsLevel == Info {
		deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Hook(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockDiscarded")
		}, event.WithWorkerPool(plugin.WorkerPool))
	} else {
		deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockScheduled")
		}, event.WithWorkerPool(plugin.WorkerPool))
		deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Hook(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockDiscarded")
		}, event.WithWorkerPool(plugin.WorkerPool))
	}
}

func configureMissingBlockMetrics(plugin *node.Plugin) {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Hook(func(block *blockdag.Block) {
		sendMissingBlockRecord(block.ModelsBlock, "missingBlock")
	}, event.WithWorkerPool(plugin.WorkerPool))
	deps.Protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Hook(func(block *blockdag.Block) {
		sendMissingBlockRecord(block.ModelsBlock, "missingBlockStored")
	}, event.WithWorkerPool(plugin.WorkerPool))
}
