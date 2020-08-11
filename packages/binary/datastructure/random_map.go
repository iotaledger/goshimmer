package datastructure

import (
	"math/rand"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type randomMapEntry struct {
	key      interface{}
	value    interface{}
	keyIndex int
}

// RandomMap defines a  map with extended ability to return a random entry.
type RandomMap struct {
	rawMap map[interface{}]*randomMapEntry
	keys   []interface{}
	size   int
	mutex  sync.RWMutex
}

// NewRandomMap creates a new random map
func NewRandomMap() *RandomMap {
	return &RandomMap{
		rawMap: make(map[interface{}]*randomMapEntry),
		keys:   make([]interface{}, 0),
	}
}

// Set associates the specified value with the specified key.
// If the association already exists, it updates the value.
func (rmap *RandomMap) Set(key interface{}, value interface{}) (updated bool) {
	rmap.mutex.Lock()

	if entry, exists := rmap.rawMap[key]; exists {
		if entry.value != value {
			entry.value = value

			updated = true
		}
	} else {
		rmap.rawMap[key] = &randomMapEntry{
			key:      key,
			value:    value,
			keyIndex: rmap.size,
		}

		updated = true

		rmap.keys = append(rmap.keys, key)

		rmap.size++
	}

	rmap.mutex.Unlock()

	return
}

// Get returns the value to which the specified key is mapped.
func (rmap *RandomMap) Get(key interface{}) (result interface{}, exists bool) {
	rmap.mutex.RLock()

	if entry, entryExists := rmap.rawMap[key]; entryExists {
		result = entry.value
		exists = entryExists
	}

	rmap.mutex.RUnlock()

	return
}

// Delete removes the mapping for the specified key in the map.
func (rmap *RandomMap) Delete(key interface{}) (result interface{}, exists bool) {
	rmap.mutex.RLock()

	if _, entryExists := rmap.rawMap[key]; entryExists {
		rmap.mutex.RUnlock()
		rmap.mutex.Lock()

		if entry, entryExists := rmap.rawMap[key]; entryExists {
			delete(rmap.rawMap, key)

			rmap.size--

			if entry.keyIndex != rmap.size {
				oldKey := entry.keyIndex
				movedKey := rmap.keys[rmap.size]

				rmap.rawMap[movedKey].keyIndex = oldKey

				rmap.keys[oldKey] = movedKey
			}

			rmap.keys = rmap.keys[:rmap.size]

			result = entry.value
			exists = true
		}

		rmap.mutex.Unlock()
	} else {
		rmap.mutex.RUnlock()
	}

	return
}

// Size returns the number of key-value mappings in the map.
func (rmap *RandomMap) Size() (result int) {
	rmap.mutex.RLock()

	result = rmap.size

	rmap.mutex.RUnlock()

	return
}

// RandomEntry returns a random value from the map.
func (rmap *RandomMap) RandomEntry() (result interface{}) {
	rmap.mutex.RLock()

	if rmap.size >= 1 {
		result = rmap.rawMap[rmap.keys[rand.Intn(rmap.size)]].value
	}

	rmap.mutex.RUnlock()

	return
}
