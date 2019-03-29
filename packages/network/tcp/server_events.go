package tcp

import (
    "github.com/iotaledger/goshimmer/packages/network"
    "reflect"
)

type serverEvents struct {
    Start    *callbackEvent
    Shutdown *callbackEvent
    Connect  *peerConsumerEvent
    Error    *errorConsumerEvent
}

// region callbackEvent /////////////////////////////////////////////////////////////////////////////////////////////////

type callbackEvent struct {
    callbacks map[uintptr]Callback
}

func (this *callbackEvent) Attach(callback Callback) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *callbackEvent) Detach(callback Callback) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *callbackEvent) Trigger() {
    for _, callback := range this.callbacks {
        callback()
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region errorConsumerEvent ////////////////////////////////////////////////////////////////////////////////////////////

type errorConsumerEvent struct {
    callbacks map[uintptr]ErrorConsumer
}

func (this *errorConsumerEvent) Attach(callback ErrorConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *errorConsumerEvent) Detach(callback ErrorConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *errorConsumerEvent) Trigger(err error) {
    for _, callback := range this.callbacks {
        callback(err)
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region peerConsumerEvent /////////////////////////////////////////////////////////////////////////////////////////////

type peerConsumerEvent struct {
    callbacks map[uintptr]PeerConsumer
}

func (this *peerConsumerEvent) Attach(callback PeerConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *peerConsumerEvent) Detach(callback PeerConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *peerConsumerEvent) Trigger(peer network.Connection) {
    for _, callback := range this.callbacks {
        callback(peer)
    }
}

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
