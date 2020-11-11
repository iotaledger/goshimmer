package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/labstack/echo"
)

var (
	webSocketWriteTimeout = time.Duration(3) * time.Second

	// clients
	wsClientsMu    sync.Mutex
	wsClients      = make(map[uint64]*wsclient)
	nextWsClientID uint64
	readHandlers   = make(map[byte]func(interface{}))

	// gorilla websocket layer
	upgrader = websocket.Upgrader{
		HandshakeTimeout:  webSocketWriteTimeout,
		CheckOrigin:       func(r *http.Request) bool { return true },
		EnableCompression: true,
	}
	shutdownSignal = make(chan os.Signal)
)

// a websocket client with a channel for downstream messages.
type wsclient struct {
	// downstream message channel.
	channel chan interface{}
	// a channel which is closed when the websocket client is disconnected.
	exit chan struct{}
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

func sendInitialData(ws *websocket.Conn) error {
	if err := manaBuffer.SendEvents(ws); err != nil {
		return err
	}
	if err := manaBuffer.SendValueMsgs(ws); err != nil {
		return err
	}
	if err := manaBuffer.SendMapOverall(ws); err != nil {
		return err
	}
	if err := manaBuffer.SendMapOnline(ws); err != nil {
		return err
	}
	return nil
}

// handles a new websocket connection, registers the client
// and waits for downstream messages to be sent to the client
func websocketRoute(c echo.Context) error {
	signal.Notify(shutdownSignal, syscall.SIGTERM)
	signal.Notify(shutdownSignal, syscall.SIGINT)

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

	manaDashboardHostAddress := config.Node().GetString(CfgManaDashboardAddress)
	registerReadHandlers(dashboard.MsgRequestManaDashboardAddress, func(_ interface{}) {
		msg := &wsmsg{
			Type: dashboard.MsgManaDashboardAddress,
			Data: manaDashboardHostAddress,
		}
		broadcastWsMessage(msg)
	})

	// replay autopeering events from the past upon connecting a new client
	analysisserver.ReplayAutopeeringEvents(createAutopeeringEventHandlers(ws))

	// replay FPC past events
	replayFPCRecords(ws)

	sendInitialData(ws)

	for {
		select {
		case msg := <-wsClient.channel:
			if err := ws.WriteJSON(msg); err != nil {
				break
			}
			if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
				break
			}
		case <-shutdownSignal:
			return nil
		default:
			_, msg, err := ws.ReadMessage()
			if err != nil {
				// silent
				break
			}
			mg := wsmsg{}
			if err := json.Unmarshal(msg, &mg); err != nil {
				log.Errorf("error unmarshalling bytes: %s", err)
				break
			}
			if f, found := readHandlers[mg.Type]; found {
				f(mg.Data)
			} else {
				log.Errorf("no handler for message type %d", msg[0])
				break
			}
		}

	}

	return nil
}

// registers a read handler for the connection.
func registerReadHandlers(msgType byte, f func(interface{})) {
	readHandlers[msgType] = f
}
