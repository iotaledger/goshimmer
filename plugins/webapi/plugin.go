package webapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the web API plugin.
const PluginName = "WebAPI"

var (
	// Plugin is the plugin instance of the web API plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	log *logger.Logger
)

type dependencies struct {
	dig.In

	Server *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
		if err := event.Container.Provide(func() *echo.Echo {
			server := newServer()
			return server
		}); err != nil {
			Plugin.Panic(err)
		}
	}))
}

// newServer creates a server instance.
func newServer() *echo.Echo {
	server := echo.New()
	server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	// if enabled, configure basic-auth
	if Parameters.BasicAuth.Enabled {
		server.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			if username == Parameters.BasicAuth.Username &&
				password == Parameters.BasicAuth.Password {
				return true, nil
			}
			return false, nil
		}))
	}

	server.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Warnf("Request failed: %s", err)

		var statusCode int
		var message string

		switch errors.Unwrap(err) {
		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "unauthorized"

		case echo.ErrForbidden:
			statusCode = http.StatusForbidden
			message = "access forbidden"

		case echo.ErrInternalServerError:
			statusCode = http.StatusInternalServerError
			message = "internal server error"

		case echo.ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case echo.ErrBadRequest:
			statusCode = http.StatusBadRequest
			message = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			message = "internal server error"
		}

		message = fmt.Sprintf("%s, error: %+v", message, err)
		resErr := c.String(statusCode, message)
		if resErr != nil {
			log.Warnf("Failed to send error response: %s", resErr)
		}
	}
	return server
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	// configure the server
	deps.Server.HideBanner = true
	deps.Server.HidePort = true
	deps.Server.GET("/", IndexRequest)
}

func run(*node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker("WebAPIServer", worker, shutdown.PriorityWebAPI); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	defer log.Infof("Stopping %s ... done", PluginName)

	stopped := make(chan struct{})
	bindAddr := Parameters.BindAddress
	go func() {
		log.Infof("%s started, bind-address=%s, basic-auth=%v", PluginName, bindAddr, Parameters.BasicAuth.Enabled)
		if err := deps.Server.Start(bindAddr); err != nil {
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
	if err := deps.Server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}
