package healthz

import (
	"net/http"
	goSync "sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPIHealthzEndpoint"

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin *node.Plugin
	once   goSync.Once

	healthy typeutils.AtomicBool
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().GET("healthz", getHealthz)
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
	Plugin().LogInfo("All plugins started successfully")
	<-shutdownSignal
}

func getHealthz(c echo.Context) error {
	if !healthy.IsSet() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusOK)
}
