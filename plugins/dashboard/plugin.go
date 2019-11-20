package dashboard

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/events"
)

var server *http.Server

var router *http.ServeMux

var PLUGIN = node.NewPlugin("Dashboard", node.Disabled, configure, run)

func configure(plugin *node.Plugin) {
	router = http.NewServeMux()
	server = &http.Server{Addr: ":8081", Handler: router}

	router.HandleFunc("/dashboard", ServeHome)
	router.HandleFunc("/ws", ServeWs)

	// send the sampledTPS to client via websocket, use uint32 to save mem
	metrics.Events.ReceivedTPSUpdated.Attach(events.NewClosure(func(sampledTPS uint64) {
		TPSQ = append(TPSQ, uint32(sampledTPS))
		if len(TPSQ) > MAX_Q_SIZE {
			TPSQ = TPSQ[1:]
		}
	}))

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
	}))
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Dashboard Updater", func() {
		go func() {
			if err := server.ListenAndServe(); err != nil {
				plugin.LogFailure(err.Error())
			}
		}()
	})
}
