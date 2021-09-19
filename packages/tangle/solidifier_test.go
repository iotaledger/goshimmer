package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestSolidifier(t *testing.T) {
	sourceTangle := NewTestTangle()
	defer sourceTangle.Shutdown()
	sourceTangle.Setup()

	sourceFramework := NewMessageTestFramework(sourceTangle)
	sourceFramework.CreateMessage("Message1", WithStrongParents("Genesis"))
	sourceFramework.CreateMessage("Message2", WithStrongParents("Genesis"))
	sourceFramework.CreateMessage("Message3", WithStrongParents("Message1"))
	sourceFramework.CreateMessage("Message4", WithStrongParents("Message2"))
	sourceFramework.CreateMessage("Message5", WithStrongParents("Message3"), WithWeakParents("Message4"))
	sourceFramework.CreateMessage("Message6", WithStrongParents("Message4"))
	sourceFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6").WaitMessagesBooked()

	destinationTangle := NewTestTangle()
	defer destinationTangle.Shutdown()
	destinationTangle.Setup()

	destinationTangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageID MessageID) {
		go func() {
			sourceTangle.Storage.Message(messageID).Consume(func(message *Message) {
				destinationTangle.Storage.StoreMessage(message)
			})
		}()
	}))

	destinationTangle.Storage.StoreMessage(sourceFramework.Message("Message5"))

	assert.Eventually(t, func() bool {
		return messagesSolid(destinationTangle, sourceFramework, "Message1", "Message3", "Message5") &&
			messagesWeaklySolid(destinationTangle, sourceFramework, "Message1", "Message3", "Message4", "Message5") &&
			!messagesExist(destinationTangle, sourceFramework, "Message2")
	}, 5*time.Minute, 100*time.Millisecond)

	destinationTangle.Storage.StoreMessage(sourceFramework.Message("Message6"))

	assert.Eventually(t, func() bool {
		return messagesSolid(destinationTangle, sourceFramework, "Message1", "Message2", "Message3", "Message4", "Message5", "Message6")
	}, 5*time.Minute, 100*time.Millisecond)
}

func messagesExist(tangle *Tangle, messageTestFramework *MessageTestFramework, aliases ...string) bool {
	for _, alias := range aliases {
		if !messageExists(tangle, messageTestFramework.Message(alias).ID()) {
			return false
		}
	}

	return true
}

func messagesSolid(tangle *Tangle, messageTestFramework *MessageTestFramework, aliases ...string) bool {
	for _, alias := range aliases {
		if !messageSolid(tangle, messageTestFramework.Message(alias).ID()) {
			return false
		}
	}

	return true
}

func messagesWeaklySolid(tangle *Tangle, messageTestFramework *MessageTestFramework, aliases ...string) bool {
	for _, alias := range aliases {
		if !messageWeaklySolid(tangle, messageTestFramework.Message(alias).ID()) {
			return false
		}
	}

	return true
}

func messageExists(tangle *Tangle, messageID MessageID) bool {
	return tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {})
}

func messageSolid(tangle *Tangle, messageID MessageID) (isSolid bool) {
	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isSolid = messageMetadata.IsSolid()
	})

	return
}

func messageWeaklySolid(tangle *Tangle, messageID MessageID) (isSolid bool) {
	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isSolid = messageMetadata.IsSolid() || messageMetadata.IsWeaklySolid()
	})

	return
}
