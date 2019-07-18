package metrics

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/websocket"
)

var (
	_, filename, _, _ = runtime.Caller(0)
	Clients           = make(map[*websocket.Conn]bool)
	templPath         = filepath.Join(filepath.Dir(filename), "./dashboard.html")
	homeTempl, _      = template.ParseFiles(templPath)
	upgrader          = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// ServeWs websocket
func ServeWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	Clients = make(map[*websocket.Conn]bool)
	Clients[ws] = true
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

	var v = struct {
		Host string
		Data []uint32
	}{
		r.Host,
		TPSQ,
	}
	homeTempl.Execute(w, &v)
}
