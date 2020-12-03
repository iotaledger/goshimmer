package webapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// PluginName is the name of the web API plugin.
const PluginName = "WebAPI"

var (
	// plugin is the plugin instance of the web API plugin.
	plugin     *node.Plugin
	pluginOnce sync.Once
	// server is the web API server.
	server     *echo.Echo
	serverOnce sync.Once

	log *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Server gets the server instance.
func Server() *echo.Echo {
	serverOnce.Do(func() {
		server = echo.New()
		server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			Skipper:      middleware.DefaultSkipper,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
		}))

		// if enabled, configure basic-auth
		if config.Node().Bool(CfgBasicAuthEnabled) {
			server.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
				if username == config.Node().String(CfgBasicAuthUsername) &&
					password == config.Node().String(CfgBasicAuthPassword) {
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
			c.String(statusCode, message)
		}
	})
	return server
}

func configure(*node.Plugin) {
	server = Server()
	log = logger.NewLogger(PluginName)
	// configure the server
	server.HideBanner = true
	server.HidePort = true
	server.GET("/", IndexRequest)
}

func run(*node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker("WebAPI server", worker, shutdown.PriorityWebAPI); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func worker(shutdownSignal <-chan struct{}) {
	defer log.Infof("Stopping %s ... done", PluginName)

	stopped := make(chan struct{})
	bindAddr := config.Node().String(CfgBindAddress)
	go func() {
		log.Infof("%s started, bind-address=%s, basic-auth=%v", PluginName, bindAddr, config.Node().Bool(CfgBasicAuthEnabled))
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
