package statusscreen_tps

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/statusscreen"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var receivedTpsCounter uint64

var solidTpsCounter uint64

var receivedTps uint64

var solidTps uint64

var PLUGIN = node.NewPlugin("Statusscreen TPS", func(plugin *node.Plugin) {
	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(_ *meta_transaction.MetaTransaction) {
		atomic.AddUint64(&receivedTpsCounter, 1)
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(_ *value_transaction.ValueTransaction) {
		atomic.AddUint64(&solidTpsCounter, 1)
	}))

	statusscreen.AddHeaderInfo(func() (s string, s2 string) {
		return "TPS", strconv.FormatUint(atomic.LoadUint64(&receivedTps), 10) + " received / " + strconv.FormatUint(atomic.LoadUint64(&solidTps), 10) + " new"
	})
}, func(plugin *node.Plugin) {
	daemon.BackgroundWorker("Statusscreen TPS Tracker", func() {
		ticker := time.NewTicker(time.Second)

		for {
			select {
			case <-daemon.ShutdownSignal:
				return

			case <-ticker.C:
				atomic.StoreUint64(&receivedTps, atomic.LoadUint64(&receivedTpsCounter))
				atomic.StoreUint64(&solidTps, atomic.LoadUint64(&solidTpsCounter))

				atomic.StoreUint64(&receivedTpsCounter, 0)
				atomic.StoreUint64(&solidTpsCounter, 0)
			}
		}
	})
})
