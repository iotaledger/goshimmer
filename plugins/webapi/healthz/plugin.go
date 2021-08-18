package healthz

import (
	"net/http"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPI healthz Endpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies

	healthy typeutils.AtomicBool
)

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	deps.Server.GET("healthz", getHealthz)
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityHealthz); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func worker(shutdownSignal <-chan struct{}) {
	// set healthy to false as soon as worker exits
	defer healthy.SetTo(false)

	healthy.SetTo(true)
	Plugin.LogInfo("All plugins started successfully")
	<-shutdownSignal
}

func getHealthz(c echo.Context) error {
	if !healthy.IsSet() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusOK)
}
