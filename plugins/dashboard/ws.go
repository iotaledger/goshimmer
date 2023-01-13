package dashboard

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
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

// a websocket client with a channel for downstream blocks.
type wsclient struct {
	// downstream block channel.
	channel chan interface{}
	// a channel which is closed when the websocket client is disconnected.
	exit chan struct{}
}

func configureWebSocketWorkerPool() {
	wsSendWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case uint64:
			broadcastWsBlock(&wsblk{MsgTypeBPSMetric, x})
			broadcastWsBlock(&wsblk{MsgTypeNodeStatus, currentNodeStatus()})
			broadcastWsBlock(&wsblk{MsgTypeNeighborMetric, neighborMetrics()})
			broadcastWsBlock(&wsblk{MsgTypeTipsMetric, &tipsInfo{
				TotalTips: deps.Protocol.TipManager.TipCount(),
			}})
		case *componentsmetric:
			broadcastWsBlock(&wsblk{MsgTypeComponentCounterMetric, x})
		case *rateSetterMetric:
			broadcastWsBlock(&wsblk{MsgTypeRateSetterMetric, x})
		}
		task.Return(nil)
	}, workerpool.WorkerCount(wsSendWorkerCount), workerpool.QueueSize(wsSendWorkerQueueSize))
}

func runWebSocketStreams() {
	updateStatus := event.NewClosure(func(event *metrics.AttachedBPSUpdatedEvent) {
		wsSendWorkerPool.TrySubmit(event.BPS)
	})
	updateComponentCounterStatus := event.NewClosure(func(event *metrics.ComponentCounterUpdatedEvent) {
		componentStatus := event.ComponentStatus
		updateStatus := &componentsmetric{
			Store:      componentStatus[metrics.Attached],
			Solidifier: componentStatus[metrics.Solidified],
			Scheduler:  componentStatus[metrics.Scheduled],
			Booker:     componentStatus[metrics.Booked],
		}
		wsSendWorkerPool.TrySubmit(updateStatus)
	})

	if err := daemon.BackgroundWorker("Dashboard[StatusUpdate]", func(ctx context.Context) {
		metrics.Events.AttachedBPSUpdated.Attach(updateStatus)
		metrics.Events.ComponentCounterUpdated.Attach(updateComponentCounterStatus)
		<-ctx.Done()
		log.Info("Stopping Dashboard[StatusUpdate] ...")
		metrics.Events.AttachedBPSUpdated.Detach(updateStatus)
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
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()

	close(wsClients[clientID].exit)
	delete(wsClients, clientID)
}

// broadcasts the given block to all connected websocket clients.
func broadcastWsBlock(blk interface{}, dontDrop ...bool) {
	wsClientsMu.RLock()
	wsClientsCopy := lo.MergeMaps(make(map[uint64]*wsclient), wsClients)
	wsClientsMu.RUnlock()
	for _, wsClient := range wsClientsCopy {
		if len(dontDrop) > 0 {
			select {
			case <-wsClient.exit:
			case wsClient.channel <- blk:
			}
			return
		}

		select {
		case <-wsClient.exit:
		case wsClient.channel <- blk:
		default:
			// potentially drop if slow consumer
		}
	}
}

func sendInitialData(ws *websocket.Conn) error {
	if err := ManaBufferInstance().SendValueBlks(ws); err != nil {
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
		var blk interface{}
		select {
		case <-wsClient.exit:
			return nil
		case blk = <-wsClient.channel:
		}

		if err := ws.WriteJSON(blk); err != nil {
			break
		}
		if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
			break
		}
	}
	return nil
}

func sendJSON(ws *websocket.Conn, blk *wsblk) error {
	if err := ws.WriteJSON(blk); err != nil {
		return err
	}
	if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
		return err
	}
	return nil
}
