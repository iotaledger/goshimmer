package ui

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/labstack/echo"
)

func configure(plugin *node.Plugin) {
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

	node.DEFAULT_LOGGER.SetEnabled(false)

	uiLogger.SetEnabled(true)
	plugin.Node.AddLogger(uiLogger)

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		node.DEFAULT_LOGGER.SetEnabled(true)
	}))
}

func run(plugin *node.Plugin) {

	webapi.Server.Static("ui", "plugins/ui")

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
