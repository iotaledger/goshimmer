package dashboard

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// PluginName is the name of the dashboard plugin.
const PluginName = "Analysis-Dashboard"

var (
	// Plugin is the plugin instance of the dashboard plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)

	log             *logger.Logger
	server *echo.Echo

	nodeStartAt = time.Now()
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	configureMetricsWorkerPool()
	configureFPCLiveFeed()
	configureAutopeeringWorkerPool()
	configureEventsRecording()
	configureServer()
}

func configureServer() {
	server = echo.New()
	server.HideBanner = true
	server.HidePort = true
	server.Use(middleware.Recover())

	if config.Node.GetBool(CfgBasicAuthEnabled) {
		server.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == config.Node.GetString(CfgBasicAuthUsername) &&
				password == config.Node.GetString(CfgBasicAuthPassword) {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(server)
}

func run(*node.Plugin) {
	// run metrics reporting
	runMetricsFeed()
	// run FPC stat reporting
	runFPCLiveFeed()
	// run data reporting for autopeering visualizer
	runAutopeeringFeed()
	// records and organizes data that was received by analysis-server
	runEventsRecordManager()

	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityAnalysis); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func worker(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", PluginName)

	// start the web socket worker pool
	metricsWorkerPool.Start()
	defer metricsWorkerPool.Stop()

	stopped := make(chan struct{})
	bindAddr := config.Node.GetString(CfgBindAddress)
	go func() {
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		if err := server.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	// stop if we are shutting down or the server could not be started
	select {
	case <-shutdownSignal:
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}

