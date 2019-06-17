package datastructure

import (
	"testing"
)

func TestAdd(t *testing.T) {
	doublyLinkedList := &DoublyLinkedList{}

	doublyLinkedList.Add(12)
	doublyLinkedList.Add(12)
	doublyLinkedList.Add(15)
	doublyLinkedList.Add(99)

	if doublyLinkedList.GetSize() != 4 {
		t.Error("the size of the list is wrong")
	}
}

func TestDelete(t *testing.T) {
	doublyLinkedList := &DoublyLinkedList{}

	doublyLinkedList.Add(12)
	doublyLinkedList.Add(13)
	doublyLinkedList.Add(15)
	doublyLinkedList.Add(99)

	if _, err := doublyLinkedList.RemoveFirst(); err != nil {
		t.Error(err)
	}

	if firstEntry, err := doublyLinkedList.GetFirst(); err != nil {
		t.Error(err)
	} else if firstEntry != 13 {
		t.Error("first entry should be 13 after delete")
	}

	if _, err := doublyLinkedList.RemoveLast(); err != nil {
		t.Error(err)
	}

	if lastEntry, err := doublyLinkedList.GetLast(); err != nil {
		t.Error(err)
	} else if lastEntry != 15 {
		t.Error("last entry should be 15 after delete")
	}

	if doublyLinkedList.GetSize() != 2 {
		t.Error("the size of the list should be 2 after delete")
	}
}
