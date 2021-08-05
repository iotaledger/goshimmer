// Package remotelogmetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotemetrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	updateTime = 500 * time.Millisecond
)

const (
	Debug uint8 = iota
	Info
	Important
	Critical
)

var (
	// plugin is the plugin instance of the remote plugin instance.
	plugin     *node.Plugin
	pluginOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("RemoteLogMetrics", node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	configureSyncMetrics()
	configureDRNGMetrics()
	configureBranchConfirmationMetrics()
	configureMessageFinalizedMetrics()
}

func run(_ *node.Plugin) {
	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Node State Logger Updater", func(shutdownSignal <-chan struct{}) {
		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() {
			checkSynced()
		}, updateTime, shutdownSignal)

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-shutdownSignal
	}, shutdown.PriorityRemoteLog); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureSyncMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	remotemetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(func(syncUpdate remotemetrics.SyncStatusChangedEvent) {
		isTangleTimeSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotemetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent))
}

func configureDRNGMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	drng.Instance().Events.Randomness.Attach(events.NewClosure(onRandomnessReceived))
}

func configureBranchConfirmationMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	}
	messagelayer.Tangle().ApprovalWeightManager.Events.BranchConfirmation.Attach(events.NewClosure(onBranchConfirmed))
}

func configureMessageFinalizedMetrics() {
	if Parameters.MetricsLevel > Info {
		return
	} else if Parameters.MetricsLevel == Info {
		messagelayer.Tangle().LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(events.NewClosure(onTransactionConfirmed))
	} else {
		messagelayer.Tangle().ApprovalWeightManager.Events.MessageFinalized.Attach(events.NewClosure(onMessageFinalized))
	}
}
