package tcp

import (
    "github.com/iotaledger/goshimmer/packages/network"
    "net"
    "strconv"
)

type Server struct {
    Socket net.Listener
    Events serverEvents
}

func (this *Server) Shutdown() {
    if this.Socket != nil {
        socket := this.Socket
        this.Socket = nil

        socket.Close()
    }
}

func (this *Server) Listen(port int) *Server {
    socket, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(port))
    if err != nil {
        this.Events.Error.Trigger(err)

        return this
    } else {
        this.Socket = socket
    }

    this.Events.Start.Trigger()
    defer this.Events.Shutdown.Trigger()

    for this.Socket != nil {
        if socket, err := this.Socket.Accept(); err != nil {
            if this.Socket != nil {
                this.Events.Error.Trigger(err)
            }
        } else {
            peer := network.NewPeer("tcp", socket)

            go this.Events.Connect.Trigger(peer)
        }
    }

    return this
}

func NewServer() *Server {
    return &Server{
        Events: serverEvents{
            Start:    &callbackEvent{make(map[uintptr]Callback)},
            Shutdown: &callbackEvent{make(map[uintptr]Callback)},
            Connect:  &peerConsumerEvent{make(map[uintptr]PeerConsumer)},
            Error:    &errorConsumerEvent{make(map[uintptr]ErrorConsumer)},
        },
    }
}
