package datastructure

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/errors"
)

type DoublyLinkedList struct {
	head  *DoublyLinkedListEntry
	tail  *DoublyLinkedListEntry
	count int
	mutex sync.RWMutex
}

// region public methods with locking //////////////////////////////////////////////////////////////////////////////////

// Appends the specified value to the end of this list.
func (list *DoublyLinkedList) Add(value interface{}) *DoublyLinkedListEntry {
	return list.AddLast(value)
}

// Appends the specified element to the end of this list.
func (list *DoublyLinkedList) AddEntry(entry *DoublyLinkedListEntry) {
	list.AddLastEntry(entry)
}

func (list *DoublyLinkedList) AddLast(value interface{}) *DoublyLinkedListEntry {
	newEntry := &DoublyLinkedListEntry{
		value: value,
	}

	list.AddLastEntry(newEntry)

	return newEntry
}

func (list *DoublyLinkedList) AddLastEntry(entry *DoublyLinkedListEntry) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	list.addLastEntry(entry)
}

func (list *DoublyLinkedList) AddFirst(value interface{}) *DoublyLinkedListEntry {
	newEntry := &DoublyLinkedListEntry{
		value: value,
	}

	list.AddFirstEntry(newEntry)

	return newEntry
}

func (list *DoublyLinkedList) AddFirstEntry(entry *DoublyLinkedListEntry) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	list.addFirstEntry(entry)
}

func (list *DoublyLinkedList) Clear() {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	list.clear()
}

func (list *DoublyLinkedList) GetFirst() (interface{}, errors.IdentifiableError) {
	if firstEntry, err := list.GetFirstEntry(); err != nil {
		return nil, err
	} else {
		return firstEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) GetFirstEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	return list.getFirstEntry()
}

func (list *DoublyLinkedList) GetLast() (interface{}, errors.IdentifiableError) {
	if lastEntry, err := list.GetLastEntry(); err != nil {
		return nil, err
	} else {
		return lastEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) GetLastEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	return list.getLastEntry()
}

func (list *DoublyLinkedList) RemoveFirst() (interface{}, errors.IdentifiableError) {
	if firstEntry, err := list.RemoveFirstEntry(); err != nil {
		return nil, err
	} else {
		return firstEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) RemoveFirstEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	return list.removeFirstEntry()
}

func (list *DoublyLinkedList) RemoveLast() (interface{}, errors.IdentifiableError) {
	if lastEntry, err := list.RemoveLastEntry(); err != nil {
		return nil, err
	} else {
		return lastEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) RemoveLastEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	return list.removeLastEntry()
}

func (list *DoublyLinkedList) Remove(value interface{}) errors.IdentifiableError {
	list.mutex.RLock()
	currentEntry := list.head
	for currentEntry != nil {
		if currentEntry.GetValue() == value {
			list.mutex.RUnlock()

			if err := list.RemoveEntry(currentEntry); err != nil {
				return err
			}

			return nil
		}

		currentEntry = currentEntry.GetNext()
	}
	list.mutex.RUnlock()

	return ErrNoSuchElement.Derive("the entry is not part of the list")
}

func (list *DoublyLinkedList) RemoveEntry(entry *DoublyLinkedListEntry) errors.IdentifiableError {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	return list.removeEntry(entry)
}

func (list *DoublyLinkedList) GetSize() int {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	return list.count
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region private methods without locking //////////////////////////////////////////////////////////////////////////////

func (list *DoublyLinkedList) addLastEntry(entry *DoublyLinkedListEntry) {
	if list.head == nil {
		list.head = entry
	} else {
		list.tail.SetNext(entry)
		entry.SetPrev(list.tail)
	}

	list.tail = entry
	list.count++
}

func (list *DoublyLinkedList) addFirstEntry(entry *DoublyLinkedListEntry) {
	if list.tail == nil {
		list.tail = entry
	} else {
		list.head.SetPrev(entry)
		entry.SetNext(list.head)
	}

	list.head = entry
	list.count++
}

func (list *DoublyLinkedList) clear() {
	list.head = nil
	list.tail = nil
	list.count = 0
}

func (list *DoublyLinkedList) getFirstEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	if list.head == nil {
		return nil, ErrNoSuchElement.Derive("the list is empty")
	}

	return list.head, nil
}

func (list *DoublyLinkedList) getLastEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	if list.tail == nil {
		return nil, ErrNoSuchElement.Derive("the list is empty")
	}

	return list.tail, nil
}

func (list *DoublyLinkedList) removeFirstEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	entryToRemove := list.head
	if err := list.removeEntry(entryToRemove); err != nil {
		return nil, err
	}

	return entryToRemove, nil
}

func (list *DoublyLinkedList) removeLastEntry() (*DoublyLinkedListEntry, errors.IdentifiableError) {
	entryToRemove := list.tail
	if err := list.removeEntry(entryToRemove); err != nil {
		return nil, err
	}

	return entryToRemove, nil
}

func (list *DoublyLinkedList) removeEntry(entry *DoublyLinkedListEntry) errors.IdentifiableError {
	if entry == nil {
		return ErrInvalidArgument.Derive("the entry must not be nil")
	}

	if list.head == nil {
		return ErrNoSuchElement.Derive("the entry is not part of the list")
	}

	nextEntry := entry.GetNext()
	if nextEntry != nil {
		nextEntry.SetPrev(entry.GetPrev())
	}
	if list.head == entry {
		list.head = nextEntry
	}

	prevEntry := entry.GetPrev()
	if prevEntry != nil {
		prevEntry.SetNext(entry.GetNext())
	}
	if list.tail == entry {
		list.tail = prevEntry
	}

	list.count--

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
