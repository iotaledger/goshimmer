// Package remotelogmetrics is a plugin that enables log metrics too complex for Prometheus, but still interesting in terms of analysis and debugging.
// It is enabled by default.
// The destination can be set via logger.remotelog.serverAddress.
package remotelogmetrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

const (
	updateTime = 500 * time.Millisecond
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
	remotelogmetrics.Events().SyncBeaconSyncChanged.Attach(events.NewClosure(func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		isSyncBeaconSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(func(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
		isTangleTimeSynced.Store(syncUpdate.CurrentStatus)
	}))
	remotelogmetrics.Events().TangleTimeSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent))
	remotelogmetrics.Events().SyncBeaconSyncChanged.Attach(events.NewClosure(sendSyncStatusChangedEvent))
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

func sendSyncStatusChangedEvent(syncUpdate remotelogmetrics.SyncStatusChangedEvent) {
	err := remotelog.RemoteLogger().Send(syncUpdate)
	if err != nil {
		plugin.Logger().Errorw("Failed to send sync status changed record on sync change event.", "err", err)
	}
}
