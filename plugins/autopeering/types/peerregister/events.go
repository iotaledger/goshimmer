package peerregister

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "reflect"
)

type peerRegisterEvents struct {
    Add    *peerEvent
    Update *peerEvent
    Remove *peerEvent
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

type PeerConsumer = func(p *peer.Peer)
