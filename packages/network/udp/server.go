package udp

import (
    "net"
)

type serverEvents struct {
    Start       *callbackEvent
    Shutdown    *callbackEvent
    ReceiveData *dataConsumerEvent
    Error       *errorConsumerEvent
}

type Server struct {
    Socket *net.UDPConn
    Events serverEvents
}

func (this *Server) Shutdown() {
    if this.Socket != nil {
        socket := this.Socket
        this.Socket = nil

        socket.Close()
    }
}

func (this *Server) Listen(address string, port int) {
    if socket, err := net.ListenUDP("udp", &net.UDPAddr{
        Port: port,
        IP:   net.ParseIP(address),
    }); err != nil {
        this.Events.Error.Trigger(err)

        return
    } else {
        this.Socket = socket
    }

    this.Events.Start.Trigger()
    defer this.Events.Shutdown.Trigger()

    buf := make([]byte, 1500)
    for this.Socket != nil {
        if bytesRead, addr, err := this.Socket.ReadFromUDP(buf); err != nil {
            if this.Socket != nil {
                this.Events.Error.Trigger(err)
            }
        } else {
            this.Events.ReceiveData.Trigger(addr, buf[:bytesRead])
        }
    }
}

func NewServer() *Server {
    return &Server{
        Events: serverEvents{
            Start:       &callbackEvent{make(map[uintptr]Callback)},
            Shutdown:    &callbackEvent{make(map[uintptr]Callback)},
            ReceiveData: &dataConsumerEvent{make(map[uintptr]AddressDataConsumer)},
            Error:       &errorConsumerEvent{make(map[uintptr]ErrorConsumer)},
        },
    }
}
