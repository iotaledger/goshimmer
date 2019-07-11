package webapi

import (
	"context"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI", configure, run)

var Server = echo.New()

func configure(plugin *node.Plugin) {
	Server.HideBanner = true
	Server.HidePort = true
	Server.GET("/", IndexRequest)

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogInfo("Stopping Web Server ...")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := Server.Shutdown(ctx); err != nil {
			plugin.LogFailure(err.Error())
		}
	}))
}

func run(plugin *node.Plugin) {
	plugin.LogInfo("Starting Web Server ...")

	daemon.BackgroundWorker("WebAPI Server", func() {
		plugin.LogSuccess("Starting Web Server ... done")

		if err := Server.Start(":8080"); err != nil {
			plugin.LogSuccess("Stopping Web Server ... done")
		}
	})
}
