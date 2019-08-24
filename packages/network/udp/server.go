package udp

import (
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/events"
)

type Server struct {
	socket            net.PacketConn
	socketMutex       sync.RWMutex
	ReceiveBufferSize int
	Events            serverEvents
}

func (this *Server) GetSocket() net.PacketConn {
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

func (this *Server) Listen(address string, port int) {
	if socket, err := net.ListenPacket("udp", address+":"+strconv.Itoa(port)); err != nil {
		this.Events.Error.Trigger(err)

		return
	} else {
		this.socketMutex.Lock()
		this.socket = socket
		this.socketMutex.Unlock()
	}

	this.Events.Start.Trigger()
	defer this.Events.Shutdown.Trigger()

	buf := make([]byte, this.ReceiveBufferSize)
	for this.GetSocket() != nil {
		if bytesRead, addr, err := this.GetSocket().ReadFrom(buf); err != nil {
			if this.GetSocket() != nil {
				this.Events.Error.Trigger(err)
			}
		} else {
			this.Events.ReceiveData.Trigger(addr.(*net.UDPAddr), buf[:bytesRead])
		}
	}
}

func NewServer(receiveBufferSize int) *Server {
	return &Server{
		ReceiveBufferSize: receiveBufferSize,
		Events: serverEvents{
			Start:       events.NewEvent(events.CallbackCaller),
			Shutdown:    events.NewEvent(events.CallbackCaller),
			ReceiveData: events.NewEvent(dataCaller),
			Error:       events.NewEvent(events.ErrorCaller),
		},
	}
}
