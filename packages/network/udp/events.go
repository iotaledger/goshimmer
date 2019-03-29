package udp

import (
    "net"
    "reflect"
)

//region peerConsumerEvent /////////////////////////////////////////////////////////////////////////////////////////////

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

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region dataConsumerEvent /////////////////////////////////////////////////////////////////////////////////////////////

type dataConsumerEvent struct {
    callbacks map[uintptr]AddressDataConsumer
}

func (this *dataConsumerEvent) Attach(callback AddressDataConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *dataConsumerEvent) Detach(callback AddressDataConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *dataConsumerEvent) Trigger(addr *net.UDPAddr, data []byte) {
    for _, callback := range this.callbacks {
        callback(addr, data)
    }
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////

//region errorConsumerEvent ////////////////////////////////////////////////////////////////////////////////////////////

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

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
