package neighborhood

import "reflect"

var Events = moduleEvents{
	Update: &callbackEvent{make(map[uintptr]Callback)},
}

type moduleEvents struct {
	Update *callbackEvent
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

type Callback = func()
