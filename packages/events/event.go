package events

type Event struct {
    triggerFunc func(handler interface{}, params ...interface{})
    callbacks   map[uintptr]interface{}
}

func (this *Event) Attach(closure *Closure) {
    this.callbacks[closure.Id] = closure.Fnc
}

func (this *Event) Detach(closure *Closure) {
    delete(this.callbacks, closure.Id)
}

func (this *Event) Trigger(params ...interface{}) {
    for _, handler := range this.callbacks {
        this.triggerFunc(handler, params...)
    }
}

func NewEvent(triggerFunc func(handler interface{}, params ...interface{})) *Event {
    return &Event{
        triggerFunc: triggerFunc,
        callbacks:   make(map[uintptr]interface{}),
    }
}
