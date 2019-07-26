package events

import "sync"

type Event struct {
	triggerFunc func(handler interface{}, params ...interface{})
	callbacks   map[uintptr]interface{}
	mutex       sync.RWMutex
}

func (this *Event) Attach(closure *Closure) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.callbacks[closure.Id] = closure.Fnc
}

func (this *Event) Detach(closure *Closure) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	delete(this.callbacks, closure.Id)
}

func (this *Event) Trigger(params ...interface{}) {
	this.mutex.RLock()
	defer this.mutex.RUnlock()
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
