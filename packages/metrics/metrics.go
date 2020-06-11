package metrics

import (
	"errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
	"sync"
)

var (
	ErrAlreadyEventRegistered = errors.New("metrics event already registered")
	ErrEventNotRegistered = errors.New("metrics event not registered")
)


type Registry struct {
	//FPCInboundBytes *events.Event
	//FPCOutboundBytes *events.Event

	definitions map[string]*events.Event
	mu syncutils.RWMutex
}

func newRegistry() *Registry{
	return &Registry{definitions: make(map[string]*events.Event)}
}

var registry *Registry
var once sync.Once

func (r *Registry) Add(name string, ev *events.Event) error{
	r.mu.Lock()
	defer r.mu.Unlock()

	// return error when already registered
	if _, exist := r.definitions[name]; exist {
		return ErrAlreadyEventRegistered
	}

	r.definitions[name] = ev
	return nil
}

func (r *Registry) Load(name string) (*events.Event, error){
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.definitions[name]; !ok {
		return nil, ErrEventNotRegistered
	}
	return r.definitions[name], nil

}

func ARegistry() *Registry{
	once.Do( func(){
		registry = newRegistry()
	})
	return registry
}