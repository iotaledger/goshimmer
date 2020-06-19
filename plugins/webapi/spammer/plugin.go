package spammer

import (
	"github.com/iotaledger/goshimmer/packages/binary/spammer"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var messageSpammer *spammer.Spammer

// PluginName is the name of the spammer plugin.
const PluginName = "Spammer"

// Plugin is the plugin instance of the spammer plugin.
var Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)

var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	messageSpammer = spammer.New(issuer.IssuePayload)
	webapi.Server.GET("spammer", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		messageSpammer.Shutdown()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
