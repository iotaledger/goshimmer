package datastructure

import (
	"sync"
)

type DoublyLinkedListEntry struct {
	value interface{}
	prev  *DoublyLinkedListEntry
	next  *DoublyLinkedListEntry
	mutex sync.RWMutex
}

func (entry *DoublyLinkedListEntry) GetNext() *DoublyLinkedListEntry {
	entry.mutex.RLock()
	defer entry.mutex.RUnlock()

	return entry.next
}

func (entry *DoublyLinkedListEntry) SetNext(next *DoublyLinkedListEntry) {
	entry.mutex.Lock()
	defer entry.mutex.Unlock()

	entry.next = next
}

func (entry *DoublyLinkedListEntry) GetPrev() *DoublyLinkedListEntry {
	entry.mutex.RLock()
	defer entry.mutex.RUnlock()

	return entry.prev
}

func (entry *DoublyLinkedListEntry) SetPrev(prev *DoublyLinkedListEntry) {
	entry.mutex.Lock()
	defer entry.mutex.Unlock()

	entry.prev = prev
}

func (entry *DoublyLinkedListEntry) GetValue() interface{} {
	entry.mutex.RLock()
	defer entry.mutex.RUnlock()

	return entry.value
}

func (entry *DoublyLinkedListEntry) SetValue(value interface{}) {
	entry.mutex.Lock()
	defer entry.mutex.Unlock()

	entry.value = value
}
