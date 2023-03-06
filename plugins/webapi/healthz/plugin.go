package healthz

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/hive.go/app/daemon"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPIHealthzEndpoint"

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Protocol *protocol.Protocol
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("healthz", getHealthz)
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityHealthz); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	Plugin.LogInfo("All plugins started successfully")
	<-ctx.Done()
}

func getHealthz(c echo.Context) error {
	if deps.Protocol.Engine().IsBootstrapped() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusOK)
}
