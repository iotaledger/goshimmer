package tangle

import (
	"container/list"
)

type Tangle struct {
	Parser         *MessageParser
	Storage        *MessageStore
	Solidifier     *Solidifier
	Booker         *MessageBooker
	Requester      *MessageRequester
	MessageFactory *MessageFactory
}

func NewTangle() (tangle *Tangle) {
	tangle = &Tangle{}
	tangle.Solidifier = NewSolidifier(tangle)

	return
}

func (t *Tangle) WalkMessageIDs(callback func(messageID MessageID) MessageIDs, entryPoints ...MessageID) {
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

func (t *Tangle) WalkMessages(callback func(message *Message, messageMetadata *MessageMetadata) MessageIDs, entryPoints ...MessageID) {
	t.WalkMessageIDs(func(messageID MessageID) (nextMessagesToVisit MessageIDs) {
		t.Storage.Message(messageID).Consume(func(message *Message) {
			t.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				nextMessagesToVisit = callback(message, messageMetadata)
			})
		})

		return
	}, entryPoints...)
}
