package dagsvisualizer

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the dags visualizer plugin.
const PluginName = "DAGsVisualizer"

var (
	deps = new(dependencies)
	// Plugin is the plugin instance of the dashboard plugin.
	Plugin *node.Plugin
	log    *logger.Logger
	server *echo.Echo
)

type dependencies struct {
	dig.In

	Tangle         *tangle.Tangle
	FinalityGadget finality.Gadget
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	configureServer()
}

func configureServer() {
	server = echo.New()
	server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))
	server.HideBanner = true
	server.HidePort = true
	server.Use(middleware.Recover())

	setupRoutes(server)
	setupVisualizer()
}

func run(plugin *node.Plugin) {
	runVisualizer()

	plugin.LogInfof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityDashboard); err != nil {
		plugin.Panicf("Error starting as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	defer log.Infof("Stopping %s ... done", PluginName)

	stopped := make(chan struct{})
	go func() {
		log.Infof("%s started, bind-address=%s", PluginName, Parameters.BindAddress)
		if err := server.Start(Parameters.BindAddress); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	// stop if we are shutting down or the server could not be started
	select {
	case <-ctx.Done():
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}
