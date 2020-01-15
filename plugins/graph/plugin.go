package graph

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"golang.org/x/net/context"

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

	newTxWorkerCount     = 1
	newTxWorkerQueueSize = 10000
	newTxWorkerPool      *workerpool.WorkerPool

	server         *http.Server
	router         *http.ServeMux
	socketioServer *socketio.Server
)

func downloadSocketIOHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, parameter.NodeConfig.GetString("graph.socketioPath"))
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
		Addr:    fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("graph.host"), parameter.NodeConfig.GetInt("graph.port")),
		Handler: router,
	}

	fs := http.FileServer(http.Dir(parameter.NodeConfig.GetString("graph.webrootPath")))

	if err := configureSocketIOServer(); err != nil {
		log.Panicf("Graph: %v", err.Error())
	}

	router.Handle("/", fs)
	router.HandleFunc("/socket.io/socket.io.js", downloadSocketIOHandler)
	router.Handle("/socket.io/", socketioServer)

	newTxWorkerPool = workerpool.New(func(task workerpool.Task) {
		onNewTx(task.Param(0).(*value_transaction.ValueTransaction))
		task.Return(nil)
	}, workerpool.WorkerCount(newTxWorkerCount), workerpool.QueueSize(newTxWorkerQueueSize))
}

func run(plugin *node.Plugin) {

	notifyNewTx := events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
		newTxWorkerPool.TrySubmit(transaction)
	})

	daemon.BackgroundWorker("Graph[NewTxWorker]", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Graph[NewTxWorker] ... done")
		tangle.Events.TransactionStored.Attach(notifyNewTx)
		newTxWorkerPool.Start()
		<-shutdownSignal
		tangle.Events.TransactionStored.Detach(notifyNewTx)
		newTxWorkerPool.Stop()
		log.Info("Stopping Graph[NewTxWorker] ... done")
	}, shutdown.ShutdownPriorityGraph)

	daemon.BackgroundWorker("Graph Webserver", func(shutdownSignal <-chan struct{}) {
		go socketioServer.Serve()

		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Error(err.Error())
			}
		}()

		log.Infof("You can now access IOTA Tangle Visualiser using: http://%s:%d", parameter.NodeConfig.GetString("graph.host"), parameter.NodeConfig.GetInt("graph.port"))

		<-shutdownSignal
		log.Info("Stopping Graph ...")

		socketioServer.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()

		_ = server.Shutdown(ctx)
		log.Info("Stopping Graph ... done")
	}, shutdown.ShutdownPriorityGraph)
}
