package tcp

import (
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/hive.go/events"
)

type Server struct {
	socket      net.Listener
	socketMutex sync.RWMutex
	Events      serverEvents
}

func (this *Server) GetSocket() net.Listener {
	this.socketMutex.RLock()
	defer this.socketMutex.RUnlock()
	return this.socket
}

func (this *Server) Shutdown() {
	this.socketMutex.Lock()
	defer this.socketMutex.Unlock()
	if this.socket != nil {
		socket := this.socket
		this.socket = nil

		socket.Close()
	}
}

func (this *Server) Listen(port int) *Server {
	socket, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		this.Events.Error.Trigger(err)

		return this
	} else {
		this.socketMutex.Lock()
		this.socket = socket
		this.socketMutex.Unlock()
	}

	this.Events.Start.Trigger()
	defer this.Events.Shutdown.Trigger()

	for this.GetSocket() != nil {
		if socket, err := this.GetSocket().Accept(); err != nil {
			if this.GetSocket() != nil {
				this.Events.Error.Trigger(err)
			}
		} else {
			peer := network.NewManagedConnection(socket)

			go this.Events.Connect.Trigger(peer)
		}
	}

	return this
}

func NewServer() *Server {
	return &Server{
		Events: serverEvents{
			Start:    events.NewEvent(events.CallbackCaller),
			Shutdown: events.NewEvent(events.CallbackCaller),
			Connect:  events.NewEvent(managedConnectionCaller),
			Error:    events.NewEvent(events.ErrorCaller),
		},
	}
}
