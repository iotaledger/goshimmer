package chosenquorum

var Events = moduleEvents{
	Update: &callbackEvent{make(map[uintptr]Callback)},
}

type moduleEvents struct {
	Update *callbackEvent
}

type callbackEvent struct {
	callbacks map[uintptr]Callback
}

func (this *callbackEvent) Trigger() {
	for _, callback := range this.callbacks {
		callback()
	}
}

type Callback = func()
