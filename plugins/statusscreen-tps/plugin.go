package statusscreen_tps

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/statusscreen"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var receivedTpsCounter uint64

var solidTpsCounter uint64

var receivedTps uint64

var solidTps uint64

var PLUGIN = node.NewPlugin("Statusscreen TPS", node.Enabled, func(plugin *node.Plugin) {
	gossip.Events.TransactionReceived.Attach(events.NewClosure(func(_ *gossip.TransactionReceivedEvent) {
		atomic.AddUint64(&receivedTpsCounter, 1)
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(_ *value_transaction.ValueTransaction) {
		atomic.AddUint64(&solidTpsCounter, 1)
	}))

	statusscreen.AddHeaderInfo(func() (s string, s2 string) {
		return "TPS", strconv.FormatUint(atomic.LoadUint64(&receivedTps), 10) + " received / " + strconv.FormatUint(atomic.LoadUint64(&solidTps), 10) + " new"
	})
}, func(plugin *node.Plugin) {
	daemon.BackgroundWorker("Statusscreen TPS Tracker", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(time.Second)

		for {
			select {
			case <-shutdownSignal:
				return

			case <-ticker.C:
				atomic.StoreUint64(&receivedTps, atomic.LoadUint64(&receivedTpsCounter))
				atomic.StoreUint64(&solidTps, atomic.LoadUint64(&solidTpsCounter))

				atomic.StoreUint64(&receivedTpsCounter, 0)
				atomic.StoreUint64(&solidTpsCounter, 0)
			}
		}
	}, shutdown.ShutdownPriorityStatusScreen)
})
