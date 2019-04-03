package udp

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "net"
    "reflect"
)

var Events = &pluginEvents{
    ReceiveRequest: &requestEvent{make(map[uintptr]ConnectionPeeringRequestConsumer)},
    ReceiveResponse: &responseEvent{make(map[uintptr]ConnectionPeeringResponseConsumer)},
    Error:          &ipErrorEvent{make(map[uintptr]IPErrorConsumer)},
}

type pluginEvents struct {
    ReceiveRequest  *requestEvent
    ReceiveResponse *responseEvent
    Error           *ipErrorEvent
}

type requestEvent struct {
    callbacks map[uintptr]ConnectionPeeringRequestConsumer
}

func (this *requestEvent) Attach(callback ConnectionPeeringRequestConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *requestEvent) Detach(callback ConnectionPeeringRequestConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *requestEvent) Trigger(request *request.Request) {
    for _, callback := range this.callbacks {
        callback(request)
    }
}

type responseEvent struct {
    callbacks map[uintptr]ConnectionPeeringResponseConsumer
}

func (this *responseEvent) Attach(callback ConnectionPeeringResponseConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *responseEvent) Detach(callback ConnectionPeeringResponseConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *responseEvent) Trigger(peeringResponse *response.Response) {
    for _, callback := range this.callbacks {
        callback(peeringResponse)
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

