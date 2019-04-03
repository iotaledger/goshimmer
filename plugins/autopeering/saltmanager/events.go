package saltmanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "reflect"
)

type packageEvents struct {
    UpdatePublicSalt  *saltEvent
    UpdatePrivateSalt *saltEvent
}

type saltEvent struct {
    callbacks map[uintptr]SaltConsumer
}

func (this *saltEvent) Attach(callback SaltConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *saltEvent) Detach(callback SaltConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *saltEvent) Trigger(salt *salt.Salt) {
    for _, callback := range this.callbacks {
        callback(salt)
    }
}

var Events = packageEvents{
    UpdatePublicSalt:  &saltEvent{make(map[uintptr]SaltConsumer)},
    UpdatePrivateSalt: &saltEvent{make(map[uintptr]SaltConsumer)},
}
