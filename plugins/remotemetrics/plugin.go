// Package remotemetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotemetrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/plugins/remotelog"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
	Tangle       *tangle.Tangle
	RemoteLogger *remotelog.RemoteLoggerConn `optional:"true"`
	DrngInstance *drng.DRNG                  `optional:"true"`
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
	measureInitialBranchCounts()
	configureSyncMetrics()
	if deps.DrngInstance != nil {
		configureDRNGMetrics()
	}
	configureBranchConfirmationMetrics()
	configureMessageFinalizedMetrics()
	configureMessageScheduledMetrics()
	configureMissingMessageMetrics()
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

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
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

func configureDRNGMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	deps.DrngInstance.Events.Randomness.Attach(event.NewClosure(func(event *drng.RandomnessEvent) { onRandomnessReceived(event.State) }))
}

func configureBranchConfirmationMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	deps.Tangle.ConfirmationOracle.Events().BranchConfirmed.Attach(event.NewClosure(func(event *tangle.BranchConfirmedEvent) {
		onBranchConfirmed(event.BranchID)
	}))

	deps.Tangle.Ledger.BranchDAG.Events.BranchCreated.Attach(event.NewClosure(func(event *branchdag.BranchCreatedEvent) {
		activeBranchesMutex.Lock()
		defer activeBranchesMutex.Unlock()

		branchID := event.BranchID
		if _, exists := activeBranches[branchID]; !exists {
			branchTotalCountDB.Inc()
			activeBranches[branchID] = types.Void
			sendBranchMetrics()
		}
	}))
}

func configureMessageFinalizedMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(event.NewClosure(func(event *tangle.TransactionConfirmedEvent) {
			onTransactionConfirmed(event.TransactionID)
		}))
	} else {
		deps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
			onMessageFinalized(event.MessageID)
		}))
	}
}

func configureMessageScheduledMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		deps.Tangle.Scheduler.Events.MessageDiscarded.Attach(event.NewClosure(func(event *tangle.MessageDiscardedEvent) {
			sendMessageSchedulerRecord(event.MessageID, "messageDiscarded")
		}))
	} else {
		deps.Tangle.Scheduler.Events.MessageScheduled.Attach(event.NewClosure(func(event *tangle.MessageScheduledEvent) {
			sendMessageSchedulerRecord(event.MessageID, "messageScheduled")
		}))
		deps.Tangle.Scheduler.Events.MessageDiscarded.Attach(event.NewClosure(func(event *tangle.MessageDiscardedEvent) {
			sendMessageSchedulerRecord(event.MessageID, "messageDiscarded")
		}))
	}
}

func configureMissingMessageMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}

	deps.Tangle.Solidifier.Events.MessageMissing.Attach(event.NewClosure(func(event *tangle.MessageMissingEvent) {
		sendMissingMessageRecord(event.MessageID, "missingMessage")

	}))
	deps.Tangle.Storage.Events.MissingMessageStored.Attach(event.NewClosure(func(event *tangle.MissingMessageStoredEvent) {
		sendMissingMessageRecord(event.MessageID, "missingMessageStored")

	}))
}
