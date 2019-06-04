package datastructure

import (
	"sync"
)

type lruCacheElement struct {
	key   interface{}
	value interface{}
}

type LRUCache struct {
	directory          map[interface{}]*DoublyLinkedListEntry
	doublyLinkedList   *DoublyLinkedList
	capacity           int
	size               int
	processingCallback bool
	mutex              sync.RWMutex
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		directory:        make(map[interface{}]*DoublyLinkedListEntry),
		doublyLinkedList: &DoublyLinkedList{},
		capacity:         capacity,
	}
}

func (cache *LRUCache) Set(key interface{}, value interface{}) {
	if !cache.processingCallback {
		cache.mutex.Lock()
		defer cache.mutex.Unlock()
	}

	element, exists := cache.directory[key]
	if exists {
		if !cache.processingCallback {
			element.GetValue().(*lruCacheElement).value = value

			cache.doublyLinkedList.mutex.Lock()
			defer cache.doublyLinkedList.mutex.Unlock()
		} else {
			element.value.(*lruCacheElement).value = value
		}

		cache.promoteElement(element)
	} else {
		cache.directory[key] = cache.doublyLinkedList.AddFirst(&lruCacheElement{key: key, value: value})

		if cache.size == cache.capacity {
			if element, err := cache.doublyLinkedList.RemoveLast(); err != nil {
				panic(err)
			} else {
				delete(cache.directory, element.(*lruCacheElement).key)
			}
		} else {
			cache.size++
		}
	}
}

func (cache *LRUCache) Contains(key interface{}, optionalCallback ...func(interface{}, bool)) bool {
	var callback func(interface{}, bool)

	if len(optionalCallback) >= 1 {
		if !cache.processingCallback {
			cache.mutex.Lock()
			defer cache.mutex.Unlock()
		}

		callback = optionalCallback[0]
	} else {
		if !cache.processingCallback {
			cache.mutex.RLock()
			defer cache.mutex.RUnlock()
		}
	}

	var elementValue interface{}
	element, exists := cache.directory[key]
	if exists {
		cache.doublyLinkedList.mutex.Lock()
		defer cache.doublyLinkedList.mutex.Unlock()

		cache.promoteElement(element)

		elementValue = element.GetValue().(*lruCacheElement).value
	}

	if callback != nil {
		cache.processingCallback = true
		callback(elementValue, exists)
		cache.processingCallback = false
	}

	return exists
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
	if !cache.processingCallback {
		cache.mutex.RLock()
	}

	entry, exists := cache.directory[key]
	if exists {
		if !cache.processingCallback {
			cache.mutex.RUnlock()

			cache.mutex.Lock()
			defer cache.mutex.Unlock()

			cache.doublyLinkedList.mutex.Lock()
			defer cache.doublyLinkedList.mutex.Unlock()
		}

		if err := cache.doublyLinkedList.removeEntry(entry); err != nil {
			panic(err)
		}
		delete(cache.directory, key)

		cache.size--

		return true
	}

	if !cache.processingCallback {
		cache.mutex.RUnlock()
	}

	return false
}

func (cache *LRUCache) promoteElement(element *DoublyLinkedListEntry) {
	if err := cache.doublyLinkedList.removeEntry(element); err != nil {
		panic(err)
	}
	cache.doublyLinkedList.addFirstEntry(element)
}
