package spammer

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/spammer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

var messageSpammer *spammer.Spammer

// PluginName is the name of the spammer plugin.
const PluginName = "Spammer"

var (
	// plugin is the plugin instance of the spammer plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	messageSpammer = spammer.New(messagelayer.Tangle().IssuePayload)
	webapi.Server().GET("spammer", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("spammer", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		messageSpammer.Shutdown()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
