package ui

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

func configure(plugin *node.Plugin) {

	//webapi.Server.Static("ui", "plugins/ui/src")
	webapi.Server.GET("ui", func(c echo.Context) error {
		return c.HTML(http.StatusOK, files["index.html"])
	})
	webapi.Server.GET("ui/**", staticFileServer)

	webapi.Server.GET("ws", upgrader)
	webapi.Server.GET("loghistory", func(c echo.Context) error {
		logMutex.RLock()
		defer logMutex.RUnlock()
		return c.JSON(http.StatusOK, logHistory)
	})
	webapi.Server.GET("tpsqueue", func(c echo.Context) error {
		tpsQueueMutex.RLock()
		defer tpsQueueMutex.RUnlock()
		return c.JSON(http.StatusOK, tpsQueue)
	})

	gossip.Events.TransactionReceived.Attach(events.NewClosure(func(_ *gossip.TransactionReceivedEvent) {
		atomic.AddUint64(&receivedTpsCounter, 1)
	}))
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(_ *value_transaction.ValueTransaction) {
		atomic.AddUint64(&solidTpsCounter, 1)
	}))
	tangle.Events.TransactionStored.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		go func() {
			saveTx(tx)
		}()
	}))

	// store log messages to send them down via the websocket
	anyMsgClosure := events.NewClosure(func(logLvl logger.Level, prefix string, msg string) {
		storeAndSendStatusMessage(logLvl, prefix, msg)
	})
	logger.Events.AnyMsg.Attach(anyMsgClosure)
}

func staticFileServer(c echo.Context) error {
	url := c.Request().URL.String()
	path := url[4:] // trim off "/ui/"
	res := c.Response()
	header := res.Header()
	if strings.HasPrefix(path, "css") {
		header.Set(echo.HeaderContentType, "text/css")
	}
	if strings.HasPrefix(path, "js") {
		header.Set(echo.HeaderContentType, "application/javascript")
	}
	return c.String(http.StatusOK, files[path])
}

func run(plugin *node.Plugin) {

	daemon.BackgroundWorker("UI Refresher", func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				return
			case <-time.After(1 * time.Second):
				wsMutex.Lock()
				ws.send(resp{
					"info": gatherInfo(),
					"txs":  logTransactions(),
				})
				wsMutex.Unlock()
			}
		}
	}, shutdown.ShutdownPriorityUI)
}

// PLUGIN plugs the UI into the main program
var PLUGIN = node.NewPlugin("UI", node.Disabled, configure, run)
