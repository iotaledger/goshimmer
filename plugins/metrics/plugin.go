package metrics

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
)

// PluginName is the name of the metrics plugin.
const PluginName = "Metrics"

var (
	// plugin is the plugin instance of the metrics plugin.
	plugin *node.Plugin
	once sync.Once
	log  *logger.Logger
)

// Plugin gets the plugin instance
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	// increase received MPS counter whenever we attached a message
	messagelayer.Tangle().Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessage.Release()
		cachedMessageMetadata.Release()
		increaseReceivedMPSCounter()
	}))
}

func run(_ *node.Plugin) {
	// create a background worker that "measures" the MPS value every second
	if err := daemon.BackgroundWorker("Metrics MPS Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureReceivedMPS, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
