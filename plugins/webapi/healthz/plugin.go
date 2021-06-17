package healthz

import (
	"net/http"
	goSync "sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the web API healthz endpoint plugin.
const PluginName = "WebAPI healthz Endpoint"

var (
	// plugin is the plugin instance of the web API info endpoint plugin.
	plugin *node.Plugin
	once   goSync.Once

	healthy typeutils.AtomicBool

	log *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().GET("healthz", getHealthz)
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityHealthz); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func worker(shutdownSignal <-chan struct{}) {
	// set healthy to false as soon as worker exits
	defer healthy.SetTo(false)

	healthy.SetTo(true)
	log.Infof("All plugins started succesfully")
	<-shutdownSignal
}

func getHealthz(c echo.Context) error {
	if !healthy.IsSet() {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusOK)
}
