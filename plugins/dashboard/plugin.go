package dashboard

import (
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"golang.org/x/net/context"
)

var (
	log        *logger.Logger
	httpServer *http.Server
)

const name = "Dashboard"

var PLUGIN = node.NewPlugin(name, node.Disabled, configure, run)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	router := http.NewServeMux()

	httpServer = &http.Server{
		Addr:    parameter.NodeConfig.GetString(CFG_BIND_ADDRESS),
		Handler: router,
	}

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

func run(*node.Plugin) {
	log.Info("Starting " + name + " ...")
	if err := daemon.BackgroundWorker(name, workerFunc, shutdown.ShutdownPriorityDashboard); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func workerFunc(shutdownSignal <-chan struct{}) {
	stopped := make(chan struct{})
	go func() {
		log.Infof("Started "+name+": http://%s/dashboard", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	select {
	case <-shutdownSignal:
	case <-stopped:
	}

	log.Info("Stopping " + name + " ...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
	log.Info("Stopping " + name + " ... done")
}
