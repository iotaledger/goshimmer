package spammer

import (
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/spammer"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

var messageSpammer *spammer.Spammer

// PluginName is the name of the spammer plugin.
const PluginName = "Spammer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   dependencies
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Tangle *tangle.Tangle
	Server *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})

	messageSpammer = spammer.New(deps.Tangle.IssuePayload, log)
	deps.Server.GET("spammer", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("spammer", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		messageSpammer.Shutdown()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
