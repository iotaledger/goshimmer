package syncutils

import (
	"fmt"
	"sync"
)

type DAGMutex[T comparable] struct {
	consumerCounter map[T]int
	mutexes         map[T]*sync.RWMutex
	sync.Mutex
}

func NewDAGMutex[T comparable]() *DAGMutex[T] {
	return &DAGMutex[T]{
		consumerCounter: make(map[T]int),
		mutexes:         make(map[T]*sync.RWMutex),
	}
}

func (d *DAGMutex[T]) RLock(ids ...T) {
	for _, mutex := range d.registerMutexes(ids...) {
		mutex.RLock()
	}
}

func (d *DAGMutex[T]) RUnlock(ids ...T) {
	for _, mutex := range d.unregisterMutexes(ids...) {
		mutex.RUnlock()
	}
}

func (d *DAGMutex[T]) Lock(id T) {
	d.Mutex.Lock()
	mutex := d.registerMutex(id)
	d.Mutex.Unlock()

	mutex.Lock()
}

func (d *DAGMutex[T]) Unlock(id T) {
	d.Mutex.Lock()
	mutex := d.unregisterMutex(id)
	if mutex == nil {
		d.Mutex.Unlock()
		return
	}
	d.Mutex.Unlock()

	mutex.Unlock()
}

func (d *DAGMutex[T]) registerMutexes(ids ...T) (mutexes []*sync.RWMutex) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	mutexes = make([]*sync.RWMutex, len(ids))
	for i, id := range ids {
		mutexes[i] = d.registerMutex(id)
	}

	return mutexes
}

func (d *DAGMutex[T]) registerMutex(id T) (mutex *sync.RWMutex) {
	mutex, mutexExists := d.mutexes[id]
	if !mutexExists {
		mutex = &sync.RWMutex{}
		d.mutexes[id] = mutex
	}

	d.consumerCounter[id]++

	return mutex
}

func (d *DAGMutex[T]) unregisterMutexes(ids ...T) (mutexes []*sync.RWMutex) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	mutexes = make([]*sync.RWMutex, 0)
	for _, id := range ids {
		if mutex := d.unregisterMutex(id); mutex != nil {
			mutexes = append(mutexes, mutex)
		}
	}

	return mutexes
}

func (d *DAGMutex[T]) unregisterMutex(id T) (mutex *sync.RWMutex) {
	if d.consumerCounter[id] == 1 {
		delete(d.consumerCounter, id)
		delete(d.mutexes, id)

		// we don't need to unlock removed mutexes as nobody else is using them anymore anyway
		return nil
	}

	mutex, mutexExists := d.mutexes[id]
	if !mutexExists {
		panic(fmt.Errorf("called Unlock or RUnlock too often for entity with %s", id))
	}

	d.consumerCounter[id]--

	return mutex
}
