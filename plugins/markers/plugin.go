package markers

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName is the name of the markers plugin.
	PluginName = "Markers"
)

var (
	plugin     *node.Plugin
	pluginOnce sync.Once
	log        *logger.Logger
)

// Plugin gets the markers plugin instance.
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
	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {

	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
