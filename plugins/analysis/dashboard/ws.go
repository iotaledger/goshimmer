package dashboard

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"

	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
)

var (
	webSocketWriteTimeout = 3 * time.Second

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

// handles a new websocket connection, registers the client
// and waits for downstream messages to be sent to the client
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

	// send mana dashboard address info
	manaDashboardHostAddress := Parameters.ManaDashboardAddress
	err = sendJSON(ws, &wsmsg{
		Type: dashboard.MsgManaDashboardAddress,
		Data: manaDashboardHostAddress,
	})
	if err != nil {
		return err
	}

	// replay autopeering events from the past upon connecting a new client
	analysisserver.ReplayAutopeeringEvents(createAutopeeringEventHandlers(ws))

	for {
		msg := <-wsClient.channel
		if err := sendJSON(ws, msg); err != nil {
			// silent
			break
		}
	}
	return nil
}

func sendJSON(ws *websocket.Conn, msg interface{}) error {
	if err := ws.WriteJSON(msg); err != nil {
		return err
	}
	if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
		return err
	}
	return nil
}
