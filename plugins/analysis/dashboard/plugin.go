package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the dashboard plugin.
const PluginName = "AnalysisDashboard"

var (
	// Plugin is the plugin instance of the dashboard plugin.
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)
	deps   = new(dependencies)
	log    *logger.Logger
	server *echo.Echo
)

type dependencies struct {
	dig.In

	Config *configuration.Configuration
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)
	configureAutopeeringWorkerPool()
	configureServer()
}

func configureServer() {
	server = echo.New()
	server.HideBanner = true
	server.HidePort = true
	server.Use(middleware.Recover())

	if Parameters.BasicAuthEnabled {
		server.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == Parameters.BasicAuthUsername &&
				password == Parameters.BasicAuthPassword {
				return true, nil
			}
			return false, nil
		}))
	}

	setupRoutes(server)
}

func run(*node.Plugin) {
	// run data reporting for autopeering visualizer
	runAutopeeringFeed()

	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker(PluginName, worker, shutdown.PriorityAnalysis); err != nil {
		log.Panicf("Error starting as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	defer log.Infof("Stopping %s ... done", PluginName)

	stopped := make(chan struct{})
	bindAddr := Parameters.BindAddress
	go func() {
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		if err := server.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	// ping all connected ws clients every second to keep the connections alive.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	// stop if we are shutting down or the server could not be started
	func() {
		for {
			select {
			case <-ticker.C:
				broadcastWsMessage(&wsmsg{MsgTypePing, ""})
			case <-ctx.Done():
				return
			case <-stopped:
				return
			}
		}
	}()

	log.Infof("Stopping %s ...", PluginName)
	stopServer()
}

func stopServer() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}
