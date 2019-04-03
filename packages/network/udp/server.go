package udp

import (
    "net"
    "strconv"
)

type serverEvents struct {
    Start       *callbackEvent
    Shutdown    *callbackEvent
    ReceiveData *dataConsumerEvent
    Error       *errorConsumerEvent
}

type Server struct {
    Socket            net.PacketConn
    ReceiveBufferSize int
    Events            serverEvents
}

func (this *Server) Shutdown() {
    if this.Socket != nil {
        socket := this.Socket
        this.Socket = nil

        socket.Close()
    }
}

func (this *Server) Listen(address string, port int) {
    if socket, err := net.ListenPacket("udp", address + ":" + strconv.Itoa(port)); err != nil {
        this.Events.Error.Trigger(err)

        return
    } else {
        this.Socket = socket
    }

    this.Events.Start.Trigger()
    defer this.Events.Shutdown.Trigger()

    buf := make([]byte, this.ReceiveBufferSize)
    for this.Socket != nil {
        if bytesRead, addr, err := this.Socket.ReadFrom(buf); err != nil {
            if this.Socket != nil {
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
            Start:       &callbackEvent{make(map[uintptr]Callback)},
            Shutdown:    &callbackEvent{make(map[uintptr]Callback)},
            ReceiveData: &dataConsumerEvent{make(map[uintptr]AddressDataConsumer)},
            Error:       &errorConsumerEvent{make(map[uintptr]ErrorConsumer)},
        },
    }
}
