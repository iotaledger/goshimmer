package webapi

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var PLUGIN = node.NewPlugin("WebAPI", node.Enabled, configure, run)
var log *logger.Logger

var Server = echo.New()

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("WebAPI")
	Server.HideBanner = true
	Server.HidePort = true
	Server.GET("/", IndexRequest)
}

func run(plugin *node.Plugin) {
	log.Info("Starting Web Server ...")

	daemon.BackgroundWorker("WebAPI Server", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Web Server ... done")

		go func() {
			if err := Server.Start(config.Node.GetString(BIND_ADDRESS)); err != nil {
				log.Info("Stopping Web Server ... done")
			}
		}()

		<-shutdownSignal

		log.Info("Stopping Web Server ...")
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := Server.Shutdown(ctx); err != nil {
			log.Errorf("Couldn't stop server cleanly: %s", err.Error())
		}
	}, shutdown.PriorityWebAPI)
}
