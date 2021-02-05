package tangle

import (
	"container/list"

	"github.com/iotaledger/hive.go/datastructure/set"
)

// Utils is a Tangle component that bundles methods that can be used to interact with the Tangle, that do not belong
// into public API.
type Utils struct {
	tangle *Tangle
}

// NewUtils is the constructor of the Utils component.
func NewUtils(tangle *Tangle) (utils *Utils) {
	return &Utils{
		tangle: tangle,
	}
}

// WalkMessageIDs is a generic Tangle walker that executes a custom callback for every visited MessageID, starting from
// the given entry points. The callback should return the MessageIDs to be visited next. It accepts an optional boolean
// parameter which can be set to true if a Message should be visited more than once following different paths.
func (u *Utils) WalkMessageIDs(callback func(messageID MessageID) (nextMessageIDsToVisit MessageIDs), entryPoints MessageIDs, revisit ...bool) {
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
// optional boolean parameter which can be set to true if a Message should be visited more than once following different
// paths.
func (u *Utils) WalkMessages(callback func(message *Message, messageMetadata *MessageMetadata) MessageIDs, entryPoints MessageIDs, revisit ...bool) {
	u.WalkMessageIDs(func(messageID MessageID) (nextMessageIDsToVisit MessageIDs) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				nextMessageIDsToVisit = callback(message, messageMetadata)
			})
		})

		return
	}, entryPoints, revisit...)
}
