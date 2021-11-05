package dashboard

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	// settings
	wsSendWorkerCount     = 1
	wsSendWorkerQueueSize = 250
	wsSendWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	webSocketWriteTimeout = 3 * time.Second

	// clients
	wsClientsMu    sync.RWMutex
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
	wsSendWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case uint64:
			broadcastWsMessage(&wsmsg{MsgTypeMPSMetric, x})
			broadcastWsMessage(&wsmsg{MsgTypeNodeStatus, currentNodeStatus()})
			broadcastWsMessage(&wsmsg{MsgTypeNeighborMetric, neighborMetrics()})
			broadcastWsMessage(&wsmsg{MsgTypeTipsMetric, &tipsInfo{
				TotalTips: deps.Tangle.TipManager.TipCount(),
			}})
		case *componentsmetric:
			broadcastWsMessage(&wsmsg{MsgTypeComponentCounterMetric, x})
		}
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))
}

func runWebSocketStreams() {
	updateStatus := events.NewClosure(func(mps uint64) {
		wsSendWorkerPool.TrySubmit(mps)
	})
	updateComponentCounterStatus := events.NewClosure(func(componentStatus map[metrics.ComponentType]uint64) {
		updateStatus := &componentsmetric{
			Store:      componentStatus[metrics.Store],
			Solidifier: componentStatus[metrics.Solidifier],
			Scheduler:  componentStatus[metrics.Scheduler],
			Booker:     componentStatus[metrics.Booker],
		}
		wsSendWorkerPool.TrySubmit(updateStatus)
	})

	if err := daemon.BackgroundWorker("Dashboard[StatusUpdate]", func(ctx context.Context) {
		metrics.Events.ReceivedMPSUpdated.Attach(updateStatus)
		metrics.Events.ComponentCounterUpdated.Attach(updateComponentCounterStatus)
		<-ctx.Done()
		log.Info("Stopping Dashboard[StatusUpdate] ...")
		metrics.Events.ReceivedMPSUpdated.Detach(updateStatus)
		wsSendWorkerPool.Stop()
		log.Info("Stopping Dashboard[StatusUpdate] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// reigsters and creates a new websocket client.
func registerWSClient() (uint64, *wsclient) {
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	clientID := nextWsClientID
	wsClient := &wsclient{
		channel: make(chan interface{}, 2000),
		exit:    make(chan struct{}),
	}
	wsClients[clientID] = wsClient
	nextWsClientID++
	return clientID, wsClient
}

// removes the websocket client with the given id.
func removeWsClient(clientID uint64) {
	wsClientsMu.RLock()
	wsClient := wsClients[clientID]
	close(wsClient.exit)
	wsClientsMu.RUnlock()

	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	delete(wsClients, clientID)
	close(wsClient.channel)
}

// broadcasts the given message to all connected websocket clients.
func broadcastWsMessage(msg interface{}, dontDrop ...bool) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
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

func sendInitialData(ws *websocket.Conn) error {
	if err := sendAllowedManaPledge(ws); err != nil {
		return err
	}
	if err := ManaBufferInstance().SendEvents(ws); err != nil {
		return err
	}
	if err := ManaBufferInstance().SendValueMsgs(ws); err != nil {
		return err
	}
	if err := ManaBufferInstance().SendMapOverall(ws); err != nil {
		return err
	}
	if err := ManaBufferInstance().SendMapOnline(ws); err != nil {
		return err
	}
	sendAllConflicts()

	return nil
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

	// send initial data to the connected client
	err = sendInitialData(ws)
	if err != nil {
		return err
	}

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

func sendJSON(ws *websocket.Conn, msg *wsmsg) error {
	if err := ws.WriteJSON(msg); err != nil {
		return err
	}
	if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
		return err
	}
	return nil
}
