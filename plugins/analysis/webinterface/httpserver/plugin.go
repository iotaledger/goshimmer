package httpserver

import (
	"errors"
	"net/http"
	"sync"
	"time"

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
	httpServer = &http.Server{Addr: ":80", Handler: router}

	router.Handle("/datastream", websocket.Handler(dataStream))
	router.HandleFunc("/", index)
}

func Run() {
	if err := daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityAnalysis); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof(name+" started: address=%s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warnf("Error listening: %s", err)
		}
	}()
	<-shutdownSignal
	log.Info("Stopping " + name + " ...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("Error closing: %s", err)
	}
	wg.Wait()
	log.Info("Stopping " + name + " ... done")
}
