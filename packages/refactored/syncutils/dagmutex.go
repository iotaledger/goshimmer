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

func (d *DAGMutex[T]) Lock(lockableEntity DAGMutexLockable[T], lockDependencies bool) {
	if lockDependencies {
		d.rLock(lockableEntity.DAGMutexDependencies()...)
	}

	d.lock(lockableEntity.DAGMutexID())
}

func (d *DAGMutex[T]) Unlock(lockableEntity DAGMutexLockable[T], lockDependencies bool) {
	d.unlock(lockableEntity.DAGMutexID())

	if lockDependencies {
		d.rUnlock(lockableEntity.DAGMutexDependencies()...)
	}
}

func (d *DAGMutex[T]) rLock(ids ...T) {
	for _, mutex := range d.registerMutexes(ids...) {
		mutex.RLock()
	}
}

func (d *DAGMutex[T]) rUnlock(ids ...T) {
	for _, mutex := range d.unregisterMutexes(ids...) {
		mutex.RUnlock()
	}
}

func (d *DAGMutex[T]) lock(id T) {
	d.Mutex.Lock()
	mutex := d.registerMutex(id)
	d.Mutex.Unlock()

	mutex.Lock()
}

func (d *DAGMutex[T]) unlock(id T) {
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

type DAGMutexLockable[T comparable] interface {
	DAGMutexID() T
	DAGMutexDependencies() []T
}
