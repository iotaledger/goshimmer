package dagsvisualizer

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo"
)

const writeTimeout = 3

var (
	// settings.
	webSocketWriteTimeout = time.Duration(writeTimeout) * time.Second

	// ws clients.
	wsClients      = make(map[uint64]*wsclient)
	nextWsClientID uint64
	wsClientsMu    sync.RWMutex
	chanLen        = 1024

	// gorilla websocket layer.
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
	log.Infof("register a client!")
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()

	clientID := nextWsClientID
	wsClient := &wsclient{
		channel: make(chan interface{}, chanLen),
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

	sendInitialData(ws)

	for {
		msg := <-wsClient.channel
		if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
			break
		}
		if err := ws.WriteJSON(msg); err != nil {
			break
		}
	}
	return nil
}

func sendInitialData(ws *websocket.Conn) {
	bufferMutex.RLock()
	defer bufferMutex.RUnlock()
	for _, msg := range buffer {
		if err := sendJSON(ws, msg); err != nil {
			log.Errorf("failed to send DAG message to client: %s", err.Error())
		}
	}
}

func sendJSON(ws *websocket.Conn, msg *wsMessage) error {
	var err error
	if err = ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err == nil {
		err = ws.WriteJSON(msg)
	}
	return err
}
