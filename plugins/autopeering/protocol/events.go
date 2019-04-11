package protocol

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "reflect"
)

var Events = protocolEvents{
    DiscoverPeer:            &peerEvent{make(map[uintptr]PeerConsumer)},
    IncomingRequestAccepted: &requestEvent{make(map[uintptr]RequestConsumer)},
    IncomingRequestRejected: &requestEvent{make(map[uintptr]RequestConsumer)},
    OutgoingRequestAccepted: &responseEvent{make(map[uintptr]ResponseConsumer)},
    OutgoingRequestRejected: &responseEvent{make(map[uintptr]ResponseConsumer)},
}

type protocolEvents struct {
    DiscoverPeer            *peerEvent
    IncomingRequestAccepted *requestEvent
    IncomingRequestRejected *requestEvent
    OutgoingRequestAccepted *responseEvent
    OutgoingRequestRejected *responseEvent
}

type peerEvent struct {
    callbacks map[uintptr]PeerConsumer
}

func (this *peerEvent) Attach(callback PeerConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *peerEvent) Detach(callback PeerConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *peerEvent) Trigger(p *peer.Peer) {
    for _, callback := range this.callbacks {
        callback(p)
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

func (this *responseEvent) Trigger(res *response.Response) {
    for _, callback := range this.callbacks {
        callback(res)
    }
}

type PeerConsumer = func(p *peer.Peer)

type RequestConsumer = func(req *request.Request)

type ResponseConsumer = func(res *response.Response)
