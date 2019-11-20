package ui

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/labstack/echo"
)

func configure(plugin *node.Plugin) {

	//webapi.Server.Static("ui", "plugins/ui/src")
	webapi.AddEndpoint("ui", func(c echo.Context) error {
		return c.HTML(http.StatusOK, files["index.html"])
	})
	webapi.AddEndpoint("ui/**", staticFileServer)

	webapi.AddEndpoint("ws", upgrader)
	webapi.AddEndpoint("loghistory", func(c echo.Context) error {
		return c.JSON(http.StatusOK, logHistory)
	})
	webapi.AddEndpoint("tpsqueue", func(c echo.Context) error {
		return c.JSON(http.StatusOK, tpsQueue)
	})

	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(_ *meta_transaction.MetaTransaction) {
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

	uiLogger.SetEnabled(true)
	plugin.Node.AddLogger(uiLogger)
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

	daemon.BackgroundWorker("UI Refresher", func() {
		for {
			select {
			case <-daemon.ShutdownSignal:
				return
			case <-time.After(1 * time.Second):
				ws.send(resp{
					"info": gatherInfo(),
					"txs":  logTransactions(),
				})
			}
		}
	})
}

// PLUGIN plugs the UI into the main program
var PLUGIN = node.NewPlugin("UI", node.Disabled, configure, run)
