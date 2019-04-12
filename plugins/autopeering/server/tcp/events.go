package tcp

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
    "net"
    "reflect"
)

var Events = &pluginEvents{
    ReceivePing:     &pingEvent{make(map[uintptr]PingConsumer)},
    ReceiveRequest:  &requestEvent{make(map[uintptr]RequestConsumer)},
    ReceiveResponse: &responseEvent{make(map[uintptr]ResponseConsumer)},
    Error:           &ipErrorEvent{make(map[uintptr]IPErrorConsumer)},
}

type pluginEvents struct {
    ReceivePing     *pingEvent
    ReceiveRequest  *requestEvent
    ReceiveResponse *responseEvent
    Error           *ipErrorEvent
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
    callbacks map[uintptr]RequestConsumer
}

func (this *requestEvent) Attach(callback RequestConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *requestEvent) Detach(callback RequestConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *requestEvent) Trigger(req *request.Request) {
    for _, callback := range this.callbacks {
        callback(req)
    }
}

type responseEvent struct {
    callbacks map[uintptr]ResponseConsumer
}

func (this *responseEvent) Attach(callback ResponseConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *responseEvent) Detach(callback ResponseConsumer) {
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
