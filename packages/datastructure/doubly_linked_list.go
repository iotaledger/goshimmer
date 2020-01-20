package datastructure

import (
	"fmt"
	"sync"
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

func (list *DoublyLinkedList) GetFirst() (interface{}, error) {
	if firstEntry, err := list.GetFirstEntry(); err != nil {
		return nil, err
	} else {
		return firstEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) GetFirstEntry() (*DoublyLinkedListEntry, error) {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	return list.getFirstEntry()
}

func (list *DoublyLinkedList) GetLast() (interface{}, error) {
	if lastEntry, err := list.GetLastEntry(); err != nil {
		return nil, err
	} else {
		return lastEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) GetLastEntry() (*DoublyLinkedListEntry, error) {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	return list.getLastEntry()
}

func (list *DoublyLinkedList) RemoveFirst() (interface{}, error) {
	if firstEntry, err := list.RemoveFirstEntry(); err != nil {
		return nil, err
	} else {
		return firstEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) RemoveFirstEntry() (*DoublyLinkedListEntry, error) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	return list.removeFirstEntry()
}

func (list *DoublyLinkedList) RemoveLast() (interface{}, error) {
	if lastEntry, err := list.RemoveLastEntry(); err != nil {
		return nil, err
	} else {
		return lastEntry.GetValue(), nil
	}
}

func (list *DoublyLinkedList) RemoveLastEntry() (*DoublyLinkedListEntry, error) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	return list.removeLastEntry()
}

func (list *DoublyLinkedList) Remove(value interface{}) error {
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

	return fmt.Errorf("%w: the entry is not part of the list", ErrNoSuchElement)
}

func (list *DoublyLinkedList) RemoveEntry(entry *DoublyLinkedListEntry) error {
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

func (list *DoublyLinkedList) getFirstEntry() (*DoublyLinkedListEntry, error) {
	if list.head == nil {
		return nil, fmt.Errorf("%w: the list is empty", ErrNoSuchElement)
	}

	return list.head, nil
}

func (list *DoublyLinkedList) getLastEntry() (*DoublyLinkedListEntry, error) {
	if list.tail == nil {
		return nil, fmt.Errorf("%w: the list is empty", ErrNoSuchElement)
	}

	return list.tail, nil
}

func (list *DoublyLinkedList) removeFirstEntry() (*DoublyLinkedListEntry, error) {
	entryToRemove := list.head
	if err := list.removeEntry(entryToRemove); err != nil {
		return nil, err
	}

	return entryToRemove, nil
}

func (list *DoublyLinkedList) removeLastEntry() (*DoublyLinkedListEntry, error) {
	entryToRemove := list.tail
	if err := list.removeEntry(entryToRemove); err != nil {
		return nil, err
	}

	return entryToRemove, nil
}

func (list *DoublyLinkedList) removeEntry(entry *DoublyLinkedListEntry) error {
	if entry == nil {
		return fmt.Errorf("%w: the entry must not be nil", ErrInvalidArgument)
	}

	if list.head == nil {
		return fmt.Errorf("%w: the entry is not part of the list", ErrNoSuchElement)
	}

	prevEntry := entry.GetPrev()
	nextEntry := entry.GetNext()

	if nextEntry != nil {
		nextEntry.SetPrev(prevEntry)
	}
	if list.head == entry {
		list.head = nextEntry
	}

	if prevEntry != nil {
		prevEntry.SetNext(nextEntry)
	}
	if list.tail == entry {
		list.tail = prevEntry
	}

	entry.SetNext(nil)
	entry.SetPrev(nil)

	list.count--

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
