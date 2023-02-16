// Package remotemetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotemetrics

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/timeutil"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

const (
	syncUpdateTime           = 500 * time.Millisecond
	schedulerQueryUpdateTime = 5 * time.Second
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
	Plugin = node.NewPlugin("RemoteLogMetrics", deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	// if remotelog plugin is disabled, then remotemetrics should not be started either
	if node.IsSkipped(remotelog.Plugin) {
		Plugin.LogInfof("%s is disabled; skipping %s\n", remotelog.Plugin.Name, Plugin.Name)
		return
	}

	configureSyncMetrics()
	configureConflictConfirmationMetrics()
	configureBlockFinalizedMetrics()
	configureBlockScheduledMetrics()
	configureMissingBlockMetrics()
	configureSchedulerQueryMetrics()
}

func run(_ *node.Plugin) {
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

func configureSyncMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	remotemetrics.Events.TangleTimeSyncChanged.Attach(event.NewClosure(func(event *remotemetrics.TangleTimeSyncChangedEvent) {
		isTangleTimeSynced.Store(event.CurrentStatus)
	}))
	remotemetrics.Events.TangleTimeSyncChanged.Attach(event.NewClosure(func(event *remotemetrics.TangleTimeSyncChangedEvent) {
		sendSyncStatusChangedEvent(event)
	}))
}

func configureSchedulerQueryMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	remotemetrics.Events.SchedulerQuery.Attach(event.NewClosure(func(event *remotemetrics.SchedulerQueryEvent) { obtainSchedulerStats(event.Time) }))
}

func configureConflictConfirmationMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(event.NewClosure(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		onConflictConfirmed(conflict.ID())
	}))

	deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictCreated.Attach(event.NewClosure(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		if !activeConflicts.Has(conflict.ID()) {
			conflictTotalCountDB.Inc()
			activeConflicts.Add(conflict.ID())
			sendConflictMetrics()
		}
	}))
}

func configureBlockFinalizedMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Attach(event.NewClosure(onTransactionAccepted))
	} else {
		deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockConfirmed.Attach(event.NewClosure(func(block *blockgadget.Block) {
			onBlockFinalized(block.ModelsBlock)
		}))
	}
}

func configureBlockScheduledMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Protocol.CongestionControl.Events.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockDiscarded")
		}))
	} else {
		deps.Protocol.CongestionControl.Events.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockScheduled")
		}))
		deps.Protocol.CongestionControl.Events.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
			sendBlockSchedulerRecord(block, "blockDiscarded")
		}))
	}
}

func configureMissingBlockMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(block *blockdag.Block) {
		sendMissingBlockRecord(block.ModelsBlock, "missingBlock")
	}))
	deps.Protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		sendMissingBlockRecord(block.ModelsBlock, "missingBlockStored")
	}))
}
