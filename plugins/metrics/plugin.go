package metrics

import (
	"encoding/binary"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

// create configure handler (get's called when the PLUGIN is "loaded" by the node)
func configure(plugin *node.Plugin) {
	// increase received TPS counter whenever we receive a new transaction
	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(_ *meta_transaction.MetaTransaction) { increaseReceivedTPSCounter() }))

	// send the sampledTPS to client via websocket, use uint32 to save mem
	Events.ReceivedTPSUpdated.Attach(events.NewClosure(func(sampledTPS uint64) {
		for client := range dashboard.Clients {
			p := make([]byte, 4)
			binary.LittleEndian.PutUint32(p, uint32(sampledTPS))
			if err := client.WriteMessage(websocket.BinaryMessage, p); err != nil {
				return
			}
			TPSQ = append(TPSQ, uint32(sampledTPS))
			if len(TPSQ) > MAX_Q_SIZE {
				TPSQ = TPSQ[1:]
			}
			dashboard.TPSQ = TPSQ
		}
	}))
}

// create run handler (get's called when the PLUGIN is "executed" by the node)
func run(plugin *node.Plugin) {
	// create a background worker that "measures" the TPS value every second
	daemon.BackgroundWorker("Metrics TPS Updater", func() { timeutil.Ticker(measureReceivedTPS, 1*time.Second) })
}

// export plugin
var PLUGIN = node.NewPlugin("Metrics", configure, run)

// TPS queue
var TPSQ []uint32
var MAX_Q_SIZE int = 3600
