package tangle

import (
	"container/list"

	"github.com/iotaledger/hive.go/kvstore"
)

// Tangle is a data structure that contains messages issued by nodes taking part in a P2P network.
type Tangle struct {
	Parser         *MessageParser
	Storage        *MessageStore
	Solidifier     *Solidifier
	Booker         *MessageBooker
	Requester      *MessageRequester
	MessageFactory *MessageFactory
	Events         *Events
}

// NewTangle is the constructor for the Tangle.
func New(store kvstore.KVStore) (tangle *Tangle) {
	tangle = &Tangle{
		Events: newEvents(),
	}
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Storage = NewMessageStore(tangle, store)

	return
}

// WalkMessageIDs is a generic Tangle walker that executes a custom callback for every visited MessageID, starting from
// the given entry points. The callback should return the MessageIDs to be visited next.
func (t *Tangle) WalkMessageIDs(callback func(messageID MessageID) (nextMessageIDsToVisit MessageIDs), entryPoints ...MessageID) {
	if len(entryPoints) == 0 {
		panic("you need to provide at least 1 entry point")
	}

	stack := list.New()
	for _, messageID := range entryPoints {
		stack.PushBack(messageID)
	}

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		for _, nextMessageID := range callback(firstElement.Value.(MessageID)) {
			stack.PushBack(nextMessageID)
		}
	}
}

// WalkMessages is generic Tangle walker that executes a custom callback for every visited Message and MessageMetadata,
// starting from the given entry points. The callback should return the MessageIDs to be visited next.
func (t *Tangle) WalkMessages(callback func(message *Message, messageMetadata *MessageMetadata) MessageIDs, entryPoints ...MessageID) {
	t.WalkMessageIDs(func(messageID MessageID) (nextMessageIDsToVisit MessageIDs) {
		t.Storage.Message(messageID).Consume(func(message *Message) {
			t.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				nextMessageIDsToVisit = callback(message, messageMetadata)
			})
		})

		return
	}, entryPoints...)
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Storage.Shutdown()
}
