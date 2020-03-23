package orderedmap

import (
	"sync"
)

type OrderedMap struct {
	head       *Element
	tail       *Element
	dictionary map[interface{}]*Element
	size       int
	mutex      sync.RWMutex
}

func New() *OrderedMap {
	return &OrderedMap{
		dictionary: make(map[interface{}]*Element),
	}
}

func (orderedMap *OrderedMap) Get(key interface{}) (interface{}, bool) {
	orderedMap.mutex.RLock()
	defer orderedMap.mutex.RUnlock()

	if orderedMapElement, orderedMapElementExists := orderedMap.dictionary[key]; !orderedMapElementExists {
		return nil, false
	} else {
		return orderedMapElement.value, true
	}
}

func (orderedMap *OrderedMap) Set(key interface{}, newValue interface{}) bool {
	if oldValue, oldValueExists := orderedMap.Get(key); oldValueExists && oldValue == newValue {
		return false
	}

	orderedMap.mutex.Lock()
	defer orderedMap.mutex.Unlock()

	if oldValue, oldValueExists := orderedMap.dictionary[key]; oldValueExists {
		if oldValue.value == newValue {
			return false
		}

		oldValue.value = newValue

		return true
	}

	newElement := &Element{
		key:   key,
		value: newValue,
	}

	if orderedMap.head == nil {
		orderedMap.head = newElement
	} else {
		orderedMap.tail.next = newElement
		newElement.prev = orderedMap.tail
	}
	orderedMap.tail = newElement
	orderedMap.size++

	orderedMap.dictionary[key] = newElement

	return true
}

func (orderedMap *OrderedMap) ForEach(consumer func(key, value interface{}) bool) bool {
	orderedMap.mutex.RLock()
	currentEntry := orderedMap.head
	orderedMap.mutex.RUnlock()

	for currentEntry != nil {
		if !consumer(currentEntry.key, currentEntry.value) {
			return false
		}

		orderedMap.mutex.RLock()
		currentEntry = currentEntry.next
		orderedMap.mutex.RUnlock()
	}

	return true
}

func (orderedMap *OrderedMap) Delete(key interface{}) bool {
	if _, valueExists := orderedMap.Get(key); !valueExists {
		return false
	}

	orderedMap.mutex.Lock()
	defer orderedMap.mutex.Unlock()

	value, valueExists := orderedMap.dictionary[key]
	if !valueExists {
		return false
	}

	delete(orderedMap.dictionary, key)
	orderedMap.size--

	if value.prev != nil {
		value.prev.next = value.next
	} else {
		orderedMap.head = value.next
	}

	if value.next != nil {
		value.next.prev = value.prev
	} else {
		orderedMap.tail = value.prev
	}

	return true
}

func (orderedMap *OrderedMap) Size() int {
	orderedMap.mutex.RLock()
	defer orderedMap.mutex.RUnlock()

	return orderedMap.size
}
