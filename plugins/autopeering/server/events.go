package server

import (
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "net"
    "reflect"
)

type pluginEvents struct {
    ReceivePeeringRequest    *addressPeeringRequestEvent
    ReceiveTCPPeeringRequest *tcpPeeringRequestEvent
    Error                    *ipErrorEvent
}

type tcpPeeringRequestEvent struct {
    callbacks map[uintptr]ConnectionPeeringRequestConsumer
}

func (this *tcpPeeringRequestEvent) Attach(callback ConnectionPeeringRequestConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *tcpPeeringRequestEvent) Detach(callback ConnectionPeeringRequestConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *tcpPeeringRequestEvent) Trigger(conn network.Connection, request *protocol.PeeringRequest) {
    for _, callback := range this.callbacks {
        callback(conn, request)
    }
}

type addressPeeringRequestEvent struct {
    callbacks map[uintptr]IPPeeringRequestConsumer
}

func (this *addressPeeringRequestEvent) Attach(callback IPPeeringRequestConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *addressPeeringRequestEvent) Detach(callback IPPeeringRequestConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *addressPeeringRequestEvent) Trigger(ip net.IP, request *protocol.PeeringRequest) {
    for _, callback := range this.callbacks {
        callback(ip, request)
    }
}

type ipErrorEvent struct {
    callbacks map[uintptr]IPErrorConsumer
}

func (this *ipErrorEvent) Attach(callback IPErrorConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *ipErrorEvent) Detach(callback IPErrorConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *ipErrorEvent) Trigger(ip net.IP, err error) {
    for _, callback := range this.callbacks {
        callback(ip, err)
    }
}

var Events = &pluginEvents{
    ReceivePeeringRequest:    &addressPeeringRequestEvent{make(map[uintptr]IPPeeringRequestConsumer)},
    ReceiveTCPPeeringRequest: &tcpPeeringRequestEvent{make(map[uintptr]ConnectionPeeringRequestConsumer)},
    Error:                    &ipErrorEvent{make(map[uintptr]IPErrorConsumer)},
}
