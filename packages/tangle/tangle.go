package tangle

import (
	"container/list"

	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
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

	// initialize components
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Storage = NewMessageStore(tangle, store)

	// initialize behavior
	tangle.Storage.Events.MessageStored.Attach(events.NewClosure(tangle.Solidifier.Solidify))

	return
}

// WalkMessageIDs is a generic Tangle walker that executes a custom callback for every visited MessageID, starting from
// the given entry points. The callback should return the MessageIDs to be visited next. It accepts an optional boolean
// parameter which can be set to true if Messages are allowed to be visited more than once through different paths.
func (t *Tangle) WalkMessageIDs(callback func(messageID MessageID) (nextMessageIDsToVisit MessageIDs), entryPoints MessageIDs, revisit ...bool) {
	if len(entryPoints) == 0 {
		panic("you need to provide at least one entry point")
	}

	stack := list.New()
	for _, messageID := range entryPoints {
		stack.PushBack(messageID)
	}

	processedMessageIDs := set.New()
	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		messageID := firstElement.Value.(MessageID)
		if (len(revisit) == 0 || !revisit[0]) && !processedMessageIDs.Add(messageID) {
			continue
		}

		for _, nextMessageID := range callback(messageID) {
			stack.PushBack(nextMessageID)
		}
	}
}

// WalkMessages is generic Tangle walker that executes a custom callback for every visited Message and MessageMetadata,
// starting from the given entry points. The callback should return the MessageIDs to be visited next. It accepts an
// optional boolean parameter which can be set to true if Messages are allowed to be visited more than once through
// different paths.
func (t *Tangle) WalkMessages(callback func(message *Message, messageMetadata *MessageMetadata) MessageIDs, entryPoints MessageIDs, revisit ...bool) {
	t.WalkMessageIDs(func(messageID MessageID) (nextMessageIDsToVisit MessageIDs) {
		t.Storage.Message(messageID).Consume(func(message *Message) {
			t.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				nextMessageIDsToVisit = callback(message, messageMetadata)
			})
		})

		return
	}, entryPoints, revisit...)
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Storage.Shutdown()
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}
