package dashboard

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/dashboardmetrics"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
)

var (
	// settings
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

func runWebSocketStreams(plugin *node.Plugin) {
	process := func(msg interface{}) {
		switch x := msg.(type) {
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
	}

	if err := daemon.BackgroundWorker("Dashboard[StatusUpdate]", func(ctx context.Context) {
		unhook := lo.Batch(
			dashboardmetrics.Events.AttachedBPSUpdated.Hook(func(event *dashboardmetrics.AttachedBPSUpdatedEvent) {
				process(event.BPS)
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,

			dashboardmetrics.Events.ComponentCounterUpdated.Hook(func(event *dashboardmetrics.ComponentCounterUpdatedEvent) {
				componentStatus := event.ComponentStatus
				process(&componentsmetric{
					Store:      componentStatus[collector.Attached],
					Solidifier: componentStatus[collector.Solidified],
					Scheduler:  componentStatus[collector.Scheduled],
					Booker:     componentStatus[collector.Booked],
				})
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,

			dashboardmetrics.Events.RateSetterUpdated.Hook(func(metric *dashboardmetrics.RateSetterMetric) {
				process(&rateSetterMetric{
					Size:     metric.Size,
					Estimate: metric.Estimate.String(),
					Rate:     metric.Rate,
				})
			}, event.WithWorkerPool(plugin.WorkerPool)).Unhook,
		)
		<-ctx.Done()
		log.Info("Stopping Dashboard[StatusUpdate] ...")
		unhook()
		log.Info("Stopping Dashboard[StatusUpdate] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// registers and creates a new websocket client.
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
