package events

import "sync"

type Event struct {
	triggerFunc func(handler interface{}, params ...interface{})
	callbacks   map[uintptr]interface{}
	mutex       sync.RWMutex
}

func (this *Event) Attach(closure *Closure) {
	this.mutex.Lock()
	this.callbacks[closure.Id] = closure.Fnc
	this.mutex.Unlock()
}

func (this *Event) Detach(closure *Closure) {
	this.mutex.Lock()
	delete(this.callbacks, closure.Id)
	this.mutex.Unlock()
}

func (this *Event) Trigger(params ...interface{}) {
	this.mutex.RLock()
	for _, handler := range this.callbacks {
		this.triggerFunc(handler, params...)
	}
	this.mutex.RUnlock()
}

func NewEvent(triggerFunc func(handler interface{}, params ...interface{})) *Event {
	return &Event{
		triggerFunc: triggerFunc,
		callbacks:   make(map[uintptr]interface{}),
	}
}
