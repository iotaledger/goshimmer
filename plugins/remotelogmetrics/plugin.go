// Package remotelogmetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/timeutil"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName is the name of the remote log plugin.
	PluginName = "RemoteLogMetrics"
)

var (
	// plugin is the plugin instance of the remote plugin instance.
	plugin     *node.Plugin
	pluginOnce sync.Once
	log        *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(plugin *node.Plugin) {
	remotelogmetrics.Events().SyncBeaconSyncChanged.Attach(events.NewClosure(func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		isSyncBeaconSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		isTangleTimeSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent()))
	remotelogmetrics.Events().SyncBeaconSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent()))

	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Node State Logger Updater", func(shutdownSignal <-chan struct{}) {
		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() {
			checkSynced()
		}, 500*time.Millisecond, shutdownSignal)

		// Wait before terminating so we get correct log messages from the daemon regarding the shutdown order.
		<-shutdownSignal
	}, shutdown.PriorityRemoteLog); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

}

func sendSyncStatusChangedEvent() func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
	return func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		err := remotelog.RemoteLogger().Send(syncUpdate)
		if err != nil {
			log.Errorw("Failed to send sync status changed record on sync change event.", "err", err)
		}
	}
}
