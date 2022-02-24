package healthz

import (
	"context"
	"net/http"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPIHealthzEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle `optional:"true"`
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
	if deps.Tangle != nil && !deps.Tangle.TimeManager.Synced() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusOK)
}
