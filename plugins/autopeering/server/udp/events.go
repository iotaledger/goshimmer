package udp

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
    "net"
    "reflect"
)

var Events = &pluginEvents{
    ReceiveDrop:     &dropEvent{make(map[uintptr]DropConsumer)},
    ReceivePing:     &pingEvent{make(map[uintptr]PingConsumer)},
    ReceiveRequest:  &requestEvent{make(map[uintptr]ConnectionPeeringRequestConsumer)},
    ReceiveResponse: &responseEvent{make(map[uintptr]ConnectionPeeringResponseConsumer)},
    Error:           &ipErrorEvent{make(map[uintptr]IPErrorConsumer)},
}

type pluginEvents struct {
    ReceiveDrop     *dropEvent
    ReceivePing     *pingEvent
    ReceiveRequest  *requestEvent
    ReceiveResponse *responseEvent
    Error           *ipErrorEvent
}

type dropEvent struct {
    callbacks map[uintptr]DropConsumer
}

func (this *dropEvent) Attach(callback DropConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *dropEvent) Detach(callback DropConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *dropEvent) Trigger(drop *drop.Drop) {
    for _, callback := range this.callbacks {
        callback(drop)
    }
}

type pingEvent struct {
    callbacks map[uintptr]PingConsumer
}

func (this *pingEvent) Attach(callback PingConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *pingEvent) Detach(callback PingConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *pingEvent) Trigger(ping *ping.Ping) {
    for _, callback := range this.callbacks {
        callback(ping)
    }
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
