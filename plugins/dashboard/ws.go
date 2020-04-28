package dashboard

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"
)

var (
	// settings
	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
	wsSendWorkerPool      *workerpool.WorkerPool
	webSocketWriteTimeout = time.Duration(3) * time.Second

	// clients
	wsClientsMu    sync.Mutex
	wsClients      = make(map[uint64]*wsclient)
	nextWsClientID uint64

	// gorilla websocket layer
	upgrader = websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true },
		EnableCompression: true,
	}
)

// a websocket client with a channel for downstream messages.
type wsclient struct {
	// downstream message channel.
	channel chan interface{}
	// a channel which is closed when the websocket client is disconnected.
	exit chan struct{}
}

func configureWebSocketWorkerPool() {
	wsSendWorkerPool = workerpool.New(func(task workerpool.Task) {
		broadcastWsMessage(&wsmsg{MsgTypeMPSMetric, task.Param(0).(uint64)})
		broadcastWsMessage(&wsmsg{MsgTypeNodeStatus, currentNodeStatus()})
		broadcastWsMessage(&wsmsg{MsgTypeNeighborMetric, neighborMetrics()})
		broadcastWsMessage(&wsmsg{MsgTypeTipsMetric, messagelayer.TipSelector.TipCount()})
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))
}

func runWebSocketStreams() {
	updateStatus := events.NewClosure(func(mps uint64) {
		wsSendWorkerPool.TrySubmit(mps)
	})

	daemon.BackgroundWorker("Dashboard[StatusUpdate]", func(shutdownSignal <-chan struct{}) {
		metrics.Events.ReceivedMPSUpdated.Attach(updateStatus)
		wsSendWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[StatusUpdate] ...")
		metrics.Events.ReceivedMPSUpdated.Detach(updateStatus)
		wsSendWorkerPool.Stop()
		log.Info("Stopping Dashboard[StatusUpdate] ... done")
	}, shutdown.PriorityDashboard)
}

// reigsters and creates a new websocket client.
func registerWSClient() (uint64, *wsclient) {
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	clientID := nextWsClientID
	wsClient := &wsclient{
		channel: make(chan interface{}, 500),
		exit:    make(chan struct{}),
	}
	wsClients[clientID] = wsClient
	nextWsClientID++
	return clientID, wsClient
}

// removes the websocket client with the given id.
func removeWsClient(clientID uint64) {
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	wsClient := wsClients[clientID]
	close(wsClient.exit)
	close(wsClient.channel)
	delete(wsClients, clientID)
}

// broadcasts the given message to all connected websocket clients.
func broadcastWsMessage(msg interface{}, dontDrop ...bool) {
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	for _, wsClient := range wsClients {
		if len(dontDrop) > 0 {
			select {
			case wsClient.channel <- msg:
			case <-wsClient.exit:
				// get unblocked if the websocket connection just got closed
			}
			continue
		}
		select {
		case wsClient.channel <- msg:
		default:
			// potentially drop if slow consumer
		}
	}
}

func websocketRoute(c echo.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered from websocket handle func: %s", r)
		}
	}()

	// upgrade to websocket connection
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()
	ws.EnableWriteCompression(true)

	// cleanup client websocket
	clientID, wsClient := registerWSClient()
	defer removeWsClient(clientID)

	for {
		msg := <-wsClient.channel
		if err := ws.WriteJSON(msg); err != nil {
			break
		}
		if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
			break
		}
	}
	return nil
}
