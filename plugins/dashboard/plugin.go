package dashboard

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"golang.org/x/net/context"
)

var server *http.Server

var router *http.ServeMux

var PLUGIN = node.NewPlugin("Dashboard", node.Disabled, configure, run)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Dashboard")
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
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Dashboard Updater", func(shutdownSignal <-chan struct{}) {
		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Error(err.Error())
			}
		}()

		<-shutdownSignal
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}, shutdown.ShutdownPriorityDashboard)
}
