package graph

import (
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"golang.org/x/net/context"

	"github.com/iotaledger/goshimmer/packages/shutdown"

	engineio "github.com/googollee/go-engine.io"
	"github.com/googollee/go-engine.io/transport"
	"github.com/googollee/go-engine.io/transport/polling"
	"github.com/googollee/go-engine.io/transport/websocket"
	socketio "github.com/googollee/go-socket.io"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	PLUGIN = node.NewPlugin("Graph", node.Disabled, configure, run)

	log *logger.Logger

	newMsgWorkerCount     = 1
	newMsgWorkerQueueSize = 10000
	newMsgWorkerPool      *workerpool.WorkerPool

	server         *http.Server
	router         *http.ServeMux
	socketioServer *socketio.Server
)

func downloadSocketIOHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, config.Node.GetString(CFG_SOCKET_IO))
}

func configureSocketIOServer() error {
	var err error

	socketioServer, err = socketio.NewServer(&engineio.Options{
		PingTimeout:  time.Second * 20,
		PingInterval: time.Second * 5,
		Transports: []transport.Transport{
			polling.Default,
			websocket.Default,
		},
	})
	if err != nil {
		return err
	}

	socketioServer.OnConnect("/", onConnectHandler)
	socketioServer.OnError("/", onErrorHandler)
	socketioServer.OnDisconnect("/", onDisconnectHandler)

	return nil
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Graph")
	initRingBuffers()

	router = http.NewServeMux()

	// socket.io and web server
	server = &http.Server{
		Addr:    config.Node.GetString(CFG_BIND_ADDRESS),
		Handler: router,
	}

	fs := http.FileServer(http.Dir(config.Node.GetString(CFG_WEBROOT)))

	if err := configureSocketIOServer(); err != nil {
		log.Panicf("Graph: %v", err.Error())
	}

	router.Handle("/", fs)
	router.HandleFunc("/socket.io/socket.io.js", downloadSocketIOHandler)
	router.Handle("/socket.io/", socketioServer)

	newMsgWorkerPool = workerpool.New(func(task workerpool.Task) {
		onAttachedMessage(task.Param(0).(*message.Message))
		task.Return(nil)
	}, workerpool.WorkerCount(newMsgWorkerCount), workerpool.QueueSize(newMsgWorkerQueueSize))
}

func run(*node.Plugin) {

	onMessageAttached := events.NewClosure(func(msg *message.Message) {
		newMsgWorkerPool.TrySubmit(msg)
	})

	daemon.BackgroundWorker("Graph[AttachedMessageWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[AttachedMessageWorker] ... done")
		messagelayer.Tangle.Events.MessageAttached.Attach(onMessageAttached)
		newMsgWorkerPool.Start()
		<-shutdownSignal
		messagelayer.Tangle.Events.MessageAttached.Detach(onMessageAttached)
		newMsgWorkerPool.Stop()
		log.Info("Stopping Graph[AttachedMessageWorker] ... done")
	}, shutdown.PriorityGraph)

	daemon.BackgroundWorker("Graph Webserver", func(shutdownSignal <-chan struct{}) {
		go socketioServer.Serve()

		stopped := make(chan struct{})
		go func() {
			log.Infof("You can now access IOTA Tangle Visualiser using: http://%s", config.Node.GetString(CFG_BIND_ADDRESS))
			if err := server.ListenAndServe(); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					log.Errorf("Error serving: %s", err)
				}
			}
			close(stopped)
		}()

		select {
		case <-shutdownSignal:
		case <-stopped:
		}

		log.Info("Stopping Graph Webserver ...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("Error stopping: %s", err)
		}

		if err := socketioServer.Close(); err != nil {
			log.Errorf("Error closing Socket.IO server: %s", err)
		}
		log.Info("Stopping Graph Webserver ... done")
	}, shutdown.PriorityGraph)
}
