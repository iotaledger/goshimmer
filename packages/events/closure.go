package events

import "reflect"

type Closure struct {
	Id  uintptr
	Fnc interface{}
}

func NewClosure(f interface{}) *Closure {
	closure := &Closure{
		Fnc: f,
	}

	closure.Id = reflect.ValueOf(closure).Pointer()

	return closure
}
