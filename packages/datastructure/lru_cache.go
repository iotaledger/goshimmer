package datastructure

import (
	"sync"

	"github.com/iotaledger/hive.go/typeutils"
)

type lruCacheElement struct {
	key   interface{}
	value interface{}
}

type LRUCache struct {
	directory        map[interface{}]*DoublyLinkedListEntry
	doublyLinkedList *DoublyLinkedList
	capacity         int
	size             int
	options          *LRUCacheOptions
	mutex            sync.RWMutex
	krwMutex         KRWMutex
}

func NewLRUCache(capacity int, options ...*LRUCacheOptions) *LRUCache {
	var currentOptions *LRUCacheOptions
	if len(options) < 1 || options[0] == nil {
		currentOptions = DEFAULT_OPTIONS
	} else {
		currentOptions = options[0]
	}

	return &LRUCache{
		directory:        make(map[interface{}]*DoublyLinkedListEntry, capacity),
		doublyLinkedList: &DoublyLinkedList{},
		capacity:         capacity,
		options:          currentOptions,
		krwMutex:         KRWMutex{keyMutexConsumers: make(map[interface{}]int), keyMutexes: make(map[interface{}]*sync.RWMutex)},
	}
}

func (cache *LRUCache) Set(key interface{}, value interface{}) {
	keyMutex := cache.krwMutex.Register(key)
	keyMutex.Lock()

	cache.mutex.Lock()
	cache.set(key, value)
	cache.mutex.Unlock()

	keyMutex.Unlock()
	cache.krwMutex.Free(key)
}

func (cache *LRUCache) set(key interface{}, value interface{}) {
	directory := cache.directory

	if element, exists := directory[key]; exists {
		element.value.(*lruCacheElement).value = value

		cache.promoteElement(element)
	} else {
		linkedListEntry := &DoublyLinkedListEntry{value: &lruCacheElement{key: key, value: value}}

		cache.doublyLinkedList.addFirstEntry(linkedListEntry)
		directory[key] = linkedListEntry

		if cache.size == cache.capacity {
			if element, err := cache.doublyLinkedList.removeLastEntry(); err != nil {
				panic(err)
			} else {
				lruCacheElement := element.value.(*lruCacheElement)
				removedElementKey := lruCacheElement.key

				delete(directory, removedElementKey)

				if cache.options.EvictionCallback != nil {
					cache.options.EvictionCallback(removedElementKey, lruCacheElement.value)
				}
			}
		} else {
			cache.size++
		}
	}
}

func (cache *LRUCache) ComputeIfAbsent(key interface{}, callback func() interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = element.GetValue().(*lruCacheElement).value

		keyMutex.RUnlock()
	} else {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()
		keyMutex.Lock()
		if result = callback(); !typeutils.IsInterfaceNil(result) {
			cache.mutex.Lock()
			cache.set(key, result)
			cache.mutex.Unlock()
		}
		keyMutex.Unlock()
	}

	cache.krwMutex.Free(key)

	return
}

// Calls the callback if an entry with the given key exists.
// The result of the callback is written back into the cache.
// If the callback returns nil the entry is removed from the cache.
// Returns the updated entry.
func (cache *LRUCache) ComputeIfPresent(key interface{}, callback func(value interface{}) interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if entry, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()
		keyMutex.Lock()

		result = entry.GetValue().(*lruCacheElement).value

		if callbackResult := callback(result); !typeutils.IsInterfaceNil(callbackResult) {
			result = callbackResult

			cache.mutex.Lock()
			cache.set(key, callbackResult)
			cache.mutex.Unlock()

			keyMutex.Unlock()
		} else {
			cache.mutex.Lock()
			if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
				panic(err)
			}
			delete(cache.directory, key)
			cache.size--
			cache.mutex.Unlock()

			keyMutex.Unlock()

			if cache.options.EvictionCallback != nil {
				cache.options.EvictionCallback(key, result)
			}
		}
	} else {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()
	}

	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) Contains(key interface{}) (result bool) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()

		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = true
	} else {
		cache.mutex.RUnlock()
		keyMutex.RUnlock()

		result = false
	}

	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) Get(key interface{}) (result interface{}) {
	keyMutex := cache.krwMutex.Register(key)

	keyMutex.RLock()
	cache.mutex.RLock()
	if element, exists := cache.directory[key]; exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		cache.promoteElement(element)
		cache.mutex.Unlock()

		result = element.GetValue().(*lruCacheElement).value

	} else {
		cache.mutex.RUnlock()
	}

	keyMutex.RUnlock()
	cache.krwMutex.Free(key)

	return
}

func (cache *LRUCache) GetCapacity() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.capacity
}

func (cache *LRUCache) GetSize() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.size
}

func (cache *LRUCache) Delete(key interface{}) bool {
	keyMutex := cache.krwMutex.Register(key)
	keyMutex.Lock()

	cache.mutex.RLock()

	entry, exists := cache.directory[key]
	if exists {
		cache.mutex.RUnlock()
		cache.mutex.Lock()
		defer cache.mutex.Unlock()

		if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
			panic(err)
		}
		delete(cache.directory, key)

		cache.size--
		keyMutex.Unlock()

		if cache.options.EvictionCallback != nil {
			cache.options.EvictionCallback(key, entry.GetValue().(*lruCacheElement).value)
		}

		return true
	}

	cache.mutex.RUnlock()

	keyMutex.Unlock()
	cache.krwMutex.Free(key)

	return false
}

func (cache *LRUCache) promoteElement(element *DoublyLinkedListEntry) {
	if err := cache.doublyLinkedList.removeEntry(element); err != nil {
		panic(err)
	}
	cache.doublyLinkedList.addFirstEntry(element)
}
