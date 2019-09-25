package visualizer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Server struct {
	router *mux.Router
}

type Event struct {
	Type   uint32 `json:"type"`
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

//var box = packr.NewBox("./templates")

var clients = make(map[*websocket.Conn]bool)
var Visualizer = make(chan *Event, 100)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewServer() *Server {
	s := &Server{}
	s.router = mux.NewRouter()
	s.router.HandleFunc("/event", eventHandler).Methods("POST")
	s.router.HandleFunc("/ws", wsHandler)
	s.router.PathPrefix("/").Handler(http.FileServer(rice.MustFindBox("frontend").HTTPBox()))
	return s
}

func (s *Server) Run() {
	go echo()

	log.Fatal(http.ListenAndServe(":8844", s.router))
}

func Writer(event *Event) {
	Visualizer <- event
}

func eventHandler(w http.ResponseWriter, r *http.Request) {
	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("ERROR: %s", err)
		http.Error(w, "Bad request", http.StatusTeapot)
		return
	}
	defer r.Body.Close()
	go Writer(&event)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	// register client
	clients[ws] = true
}

func echo() {
	for {
		val := <-Visualizer
		time.Sleep(50 * time.Millisecond)
		event := fmt.Sprintf("%d %s %s", val.Type, val.Source, val.Dest)

		// send to every client that is currently connected
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(event))
			if err != nil {
				log.Printf("Websocket error: %s", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
