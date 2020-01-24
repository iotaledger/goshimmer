package httpserver

import (
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

var (
	log        *logger.Logger
	httpServer *http.Server
	router     *http.ServeMux
)

const name = "Analysis HTTP Server"

func Configure() {
	log = logger.NewLogger(name)

	router = http.NewServeMux()

	httpServer = &http.Server{
		Addr:    parameter.NodeConfig.GetString(CFG_BIND_ADDRESS),
		Handler: router,
	}

	router.Handle("/datastream", websocket.Handler(dataStream))
	router.HandleFunc("/", index)
}

func Run() {
	log.Infof("Starting %s ...", name)
	if err := daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityAnalysis); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	stopped := make(chan struct{})
	go func() {
		log.Infof("Started %s: http://%s", name, httpServer.Addr)
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

	log.Infof("Stopping %s ...", name)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
	log.Info("Stopping %s ... done", name)
}
