// Package remotemetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotemetrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
	"github.com/iotaledger/goshimmer/plugins/remotelog"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/iotaledger/hive.go/core/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
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
	Tangle       *tangleold.Tangle
	RemoteLogger *remotelog.RemoteLoggerConn `optional:"true"`
	ClockPlugin  *node.Plugin                `name:"clock" optional:"true"`
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
	measureInitialConflictCounts()
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
		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() { checkSynced() }, syncUpdateTime, ctx)
		timeutil.NewTicker(func() { remotemetrics.Events.SchedulerQuery.Trigger(&remotemetrics.SchedulerQueryEvent{time.Now()}) }, schedulerQueryUpdateTime, ctx)

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
	deps.Tangle.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		onConflictConfirmed(event.ID)
	}))

	deps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		activeConflictsMutex.Lock()
		defer activeConflictsMutex.Unlock()

		conflictID := event.ID
		if _, exists := activeConflicts[conflictID]; !exists {
			conflictTotalCountDB.Inc()
			activeConflicts[conflictID] = types.Void
			sendConflictMetrics()
		}
	}))
}

func configureBlockFinalizedMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Tangle.Ledger.Events.TransactionAccepted.Attach(event.NewClosure(func(event *ledger.TransactionAcceptedEvent) {
			onTransactionConfirmed(event.TransactionID)
		}))
	} else {
		deps.Tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
			onBlockFinalized(event.Block)
		}))
	}
}

func configureBlockScheduledMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Tangle.Scheduler.Events.BlockDiscarded.Attach(event.NewClosure(func(event *tangleold.BlockDiscardedEvent) {
			sendBlockSchedulerRecord(event.BlockID, "blockDiscarded")
		}))
	} else {
		deps.Tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *tangleold.BlockScheduledEvent) {
			sendBlockSchedulerRecord(event.BlockID, "blockScheduled")
		}))
		deps.Tangle.Scheduler.Events.BlockDiscarded.Attach(event.NewClosure(func(event *tangleold.BlockDiscardedEvent) {
			sendBlockSchedulerRecord(event.BlockID, "blockDiscarded")
		}))
	}
}

func configureMissingBlockMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Tangle.Solidifier.Events.BlockMissing.Attach(event.NewClosure(func(event *tangleold.BlockMissingEvent) {
		sendMissingBlockRecord(event.BlockID, "missingBlock")
	}))
	deps.Tangle.Storage.Events.MissingBlockStored.Attach(event.NewClosure(func(event *tangleold.MissingBlockStoredEvent) {
		sendMissingBlockRecord(event.BlockID, "missingBlockStored")
	}))
}
