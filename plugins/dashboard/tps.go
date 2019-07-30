package dashboard

import (
	"encoding/binary"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	homeTempl, templ_err = template.New("dashboard").Parse(tpsTemplate)
	upgrader             = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// ServeWs websocket
func ServeWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	notifyWebsocketClient := events.NewClosure(func(sampledTPS uint64) {
		p := make([]byte, 4)
		binary.LittleEndian.PutUint32(p, uint32(sampledTPS))
		if err := ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
			return
		}
	})

	metrics.Events.ReceivedTPSUpdated.Attach(notifyWebsocketClient)

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}

	fmt.Println("DISCONNECTOR")

	metrics.Events.ReceivedTPSUpdated.Detach(notifyWebsocketClient)
}

// ServeHome registration
func ServeHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/dashboard" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if templ_err != nil {
		panic("HTML template error")
	}

	_ = homeTempl.Execute(w, &struct {
		Host string
		Data []uint32
	}{
		r.Host,
		TPSQ,
	})
}

var TPSQ []uint32

const (
	MAX_Q_SIZE int = 3600
)
