package datastructure

import (
	"sync"
)

type KRWMutex struct {
	keyMutexConsumers map[interface{}]int
	keyMutexes        map[interface{}]*sync.RWMutex
	mutex             sync.RWMutex
}

func NewKRWMutex() *KRWMutex {
	return &KRWMutex{
		keyMutexConsumers: make(map[interface{}]int),
		keyMutexes:        make(map[interface{}]*sync.RWMutex),
	}
}

func (krwMutex *KRWMutex) Register(key interface{}) (result *sync.RWMutex) {
	krwMutex.mutex.Lock()

	if val, exists := krwMutex.keyMutexConsumers[key]; exists {
		krwMutex.keyMutexConsumers[key] = val + 1
		result = krwMutex.keyMutexes[key]
	} else {
		result = &sync.RWMutex{}

		krwMutex.keyMutexConsumers[key] = 1
		krwMutex.keyMutexes[key] = result
	}

	krwMutex.mutex.Unlock()

	return
}

func (kwrMutex *KRWMutex) Free(key interface{}) {
	kwrMutex.mutex.Lock()

	if val, exists := kwrMutex.keyMutexConsumers[key]; exists {
		if val == 1 {
			delete(kwrMutex.keyMutexConsumers, key)
			delete(kwrMutex.keyMutexes, key)
		} else {
			kwrMutex.keyMutexConsumers[key] = val - 1
		}
	} else {
		panic("trying to free non-existent key")
	}

	kwrMutex.mutex.Unlock()
}
