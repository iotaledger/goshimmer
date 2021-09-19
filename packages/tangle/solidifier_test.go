package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
)

func TestSolidifier(t *testing.T) {
	sourceTangle := NewTestTangle()
	defer sourceTangle.Shutdown()

	sourceFramework := NewMessageTestFramework(sourceTangle)

	sourceTangle.Setup()

	sourceFramework.CreateMessage("Message1", WithStrongParents("Genesis"))
	sourceFramework.CreateMessage("Message2", WithStrongParents("Genesis"))
	sourceFramework.CreateMessage("Message3", WithStrongParents("Message1"))
	sourceFramework.CreateMessage("Message4", WithStrongParents("Message2"))
	sourceFramework.CreateMessage("Message5", WithStrongParents("Message3"), WithWeakParents("Message4"))
	sourceFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5").WaitMessagesBooked()

	destinationTangle := NewTestTangle()
	defer destinationTangle.Shutdown()
	destinationTangle.Setup()

	destinationTangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("MISSING", messageID)
		go func() {
			sourceTangle.Storage.Message(messageID).Consume(func(message *Message) {
				destinationTangle.Storage.StoreMessage(message)
			})
		}()
	}))

	destinationTangle.Parser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		fmt.Println("MessageParsed", msgParsedEvent.Message)
	}))

	destinationTangle.Parser.Events.BytesRejected.Attach(events.NewClosure(func(bytesRejectedEvent *BytesRejectedEvent, err error) {
		fmt.Println("BytesRejected", bytesRejectedEvent.Bytes)
	}))

	destinationTangle.Parser.Events.MessageRejected.Attach(events.NewClosure(func(messageRejectedEvent *MessageRejectedEvent, err error) {
		fmt.Println("MessageRejected", messageRejectedEvent.Message, err)
	}))

	destinationTangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("MessageSolid", messageID)
	}))

	destinationTangle.Solidifier.Events.MessageWeaklySolid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("MessageWeaklySolid", messageID)
	}))

	fmt.Println("PROCESS")

	destinationTangle.Storage.StoreMessage(sourceFramework.Message("Message5"))

	time.Sleep(5 * time.Second)
}
