// Package remotelogmetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

const (
	updateTime = 500 * time.Millisecond
)

var (
	// Plugin is the plugin instance of the remote plugin instance.
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In

	Local              *peer.Local
	Tangle             *tangle.Tangle
	Voter              vote.DRNGRoundBasedVoter
	RemoteLogger       *remotelog.RemoteLoggerConn
	DrngInstance       *drng.DRNG
	ClockPlugin        *node.Plugin `name:"clock" optional:"true"`
	ConsensusMechanism tangle.ConsensusMechanism
}

func init() {
	Plugin = node.NewPlugin("RemoteLogMetrics", node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	configureSyncMetrics()
	configureFPCConflictsMetrics()
	configureDRNGMetrics()
	configureTransactionMetrics()
	configureStatementMetrics()
}

func run(plugin *node.Plugin) {
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
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		isTangleTimeSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent))
}

func sendSyncStatusChangedEvent(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
	err := deps.RemoteLogger.Send(syncUpdate)
	if err != nil {
		Plugin.Logger().Errorw("Failed to send sync status changed record on sync change event.", "err", err)
	}
}

func configureFPCConflictsMetrics() {
	deps.Voter.Events().Finalized.Attach(events.NewClosure(onVoteFinalized))
	deps.Voter.Events().RoundExecuted.Attach(events.NewClosure(onVoteRoundExecuted))
}

func configureDRNGMetrics() {
	deps.DrngInstance.Events.Randomness.Attach(events.NewClosure(onRandomnessReceived))
}

func configureTransactionMetrics() {
	deps.Tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(onTransactionOpinionFormed))
	deps.Tangle.LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(events.NewClosure(onTransactionConfirmed))
}

func configureStatementMetrics() {
	deps.Tangle.ConsensusManager.Events.StatementProcessed.Attach(events.NewClosure(onStatementReceived))
}
