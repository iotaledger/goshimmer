package daemon

import "reflect"

type daemonEvents struct {
    Run      *callbackEvent
    Shutdown *callbackEvent
}

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
