package syncutils

import (
	"fmt"
	"sync"
)

type DAGMutexLockable interface {
	DAGMutexID() DAGMutexID
	DAGMutexDependencies() []DAGMutexID
}

type DAGMutex struct {
	keyMutexConsumers map[DAGMutexID]int
	keyMutexes        map[DAGMutexID]*sync.RWMutex
	mutex             sync.Mutex
}

func (d *DAGMutex) LockEntity(lockableEntity DAGMutexLockable) {
	d.RLock(lockableEntity.DAGMutexDependencies()...)
	d.Lock(lockableEntity.DAGMutexID())
}

func (d *DAGMutex) UnlockEntity(lockableEntity DAGMutexLockable) {
	d.Unlock(lockableEntity.DAGMutexID())
	d.RUnlock(lockableEntity.DAGMutexDependencies()...)
}

func (d *DAGMutex) RLock(ids ...DAGMutexID) {
	for _, mutex := range d.registerMutexes(ids...) {
		mutex.RLock()
	}
}

func (d *DAGMutex) RUnlock(ids ...DAGMutexID) {
	for _, mutex := range d.unregisterMutexes(ids...) {
		mutex.RUnlock()
	}
}

func (d *DAGMutex) Lock(id DAGMutexID) {
	d.mutex.Lock()
	mutex := d.registerMutex(id)
	d.mutex.Unlock()

	mutex.Lock()
}

func (d *DAGMutex) Unlock(id DAGMutexID) {
	d.mutex.Lock()
	mutex := d.unregisterMutex(id)
	if mutex == nil {
		d.mutex.Unlock()
		return
	}
	d.mutex.Unlock()

	mutex.Unlock()
}

func (d *DAGMutex) registerMutexes(ids ...DAGMutexID) (mutexes []*sync.RWMutex) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	mutexes = make([]*sync.RWMutex, len(ids))
	for i, id := range ids {
		mutexes[i] = d.registerMutex(id)
	}

	return mutexes
}

func (d *DAGMutex) registerMutex(id DAGMutexID) (mutex *sync.RWMutex) {
	mutex, mutexExists := d.keyMutexes[id]
	if !mutexExists {
		mutex = &sync.RWMutex{}
		d.keyMutexes[id] = mutex
	}

	d.keyMutexConsumers[id]++

	return mutex
}

func (d *DAGMutex) unregisterMutexes(ids ...DAGMutexID) (mutexes []*sync.RWMutex) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	mutexes = make([]*sync.RWMutex, 0)
	for _, id := range ids {
		if mutex := d.unregisterMutex(id); mutex != nil {
			mutexes = append(mutexes, mutex)
		}
	}

	return mutexes
}

func (d *DAGMutex) unregisterMutex(id DAGMutexID) (mutex *sync.RWMutex) {
	if d.keyMutexConsumers[id] == 1 {
		delete(d.keyMutexConsumers, id)
		delete(d.keyMutexes, id)

		// we don't need to unlock removed mutexes as nobody else is using them anymore anyway
		return nil
	}

	mutex, mutexExists := d.keyMutexes[id]
	if !mutexExists {
		panic(fmt.Errorf("called Unlock or RUnlock too often for entity with %s", id))
	}

	d.keyMutexConsumers[id]--

	return mutex
}

type DAGMutexID [32]byte
