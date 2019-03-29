package node

import (
    "reflect"
)

type pluginEvents struct {
    Configure *callbackEvent
    Run       *callbackEvent
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

func (this *callbackEvent) Trigger(plugin *Plugin) {
    for _, callback := range this.callbacks {
        callback(plugin)
    }
}
