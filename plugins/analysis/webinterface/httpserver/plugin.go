package httpserver

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

var (
	httpServer *http.Server
	router     *http.ServeMux
)

func Configure(plugin *node.Plugin) {
	router = http.NewServeMux()
	httpServer = &http.Server{Addr: ":80", Handler: router}

	router.Handle("/datastream", websocket.Handler(dataStream))
	router.HandleFunc("/", index)
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Analysis HTTP Server", func(shutdownSignal <-chan struct{}) {
		go httpServer.ListenAndServe()
		<-shutdownSignal
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}, shutdown.ShutdownPriorityAnalysis)
}
