package spammer

import (
	"context"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/app/spammer"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

var blockSpammer *spammer.Spammer

// PluginName is the name of the spammer plugin.
const PluginName = "Spammer"

var (
	// Plugin is the plugin instance of the spammer plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Tangle *tangleold.Tangle
	Server *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	blockSpammer = spammer.New(deps.Tangle.IssuePayload, log, deps.Tangle.RateSetter.Estimate)
	deps.Server.GET("spammer", handleRequest)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("spammer", func(ctx context.Context) {
		<-ctx.Done()

		blockSpammer.Shutdown()
	}, shutdown.PrioritySpammer); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
