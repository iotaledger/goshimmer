package dashboard

import (
	"log"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var PLUGIN = node.NewPlugin("Dashboard", configure, run)

func configure(plugin *node.Plugin) {
	http.HandleFunc("/dashboard", ServeHome)
	http.HandleFunc("/ws", ServeWs)

	// send the sampledTPS to client via websocket, use uint32 to save mem
	metrics.Events.ReceivedTPSUpdated.Attach(events.NewClosure(func(sampledTPS uint64) {
		TPSQ = append(TPSQ, uint32(sampledTPS))
		if len(TPSQ) > MAX_Q_SIZE {
			TPSQ = TPSQ[1:]
		}
	}))
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Dashboard Updater", func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatal(err)
		}
	})
}
