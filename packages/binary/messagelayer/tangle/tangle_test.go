package tangle

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkTangle_AttachMessage(b *testing.B) {
	tangle := New(mapdb.NewMapDB())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	messageBytes := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestMessage("some data")
		messageBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.AttachMessage(messageBytes[i])
	}

	tangle.Shutdown()
}

func TestTangle_AttachMessage(t *testing.T) {
	messageTangle := New(mapdb.NewMapDB())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			fmt.Println("ATTACHED:", msg.ID())
		})
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			fmt.Println("SOLID:", msg.ID())
		})
	}))

	messageTangle.Events.MessageUnsolidifiable.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("UNSOLIDIFIABLE:", messageId)
	}))

	messageTangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.Events.MessageRemoved.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("REMOVED:", messageId)
	}))

	newMessageOne := newTestMessage("some data")
	newMessageTwo := newTestMessage("some other data")

	messageTangle.AttachMessage(newMessageTwo)

	time.Sleep(7 * time.Second)

	messageTangle.AttachMessage(newMessageOne)

	messageTangle.Shutdown()
}

func TestTangle_MissingMessages(t *testing.T) {
	// test parameters
	messageCount := 10000

	// variables required for the test
	previousMessageID := message.EmptyID
	missingMessagesCounter := int32(0)
	wg := sync.WaitGroup{}

	// setup the message factory
	msgFactory := messagefactory.New(
		mapdb.NewMapDB(),
		[]byte("sequenceKey"),
		identity.GenerateLocalIdentity(),
		messagefactory.TipSelectorFunc(func() (message.ID, message.ID) { return previousMessageID, previousMessageID }),
	)
	defer msgFactory.Shutdown()

	// create a helper function that creates the messages
	createNewMessage := func() *message.Message {
		// issue the payload
		msg, err := msgFactory.IssuePayload(payload.NewData([]byte("0")))
		require.NoError(t, err)

		// link the new message to the previous one
		previousMessageID = msg.ID()

		// return the constructed message
		return msg
	}

	// create the tangle
	tangle := New(mapdb.NewMapDB())
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	// generate the messages we want to solidify
	preGeneratedMessages := make([]*message.Message, messageCount)
	for i := 0; i < messageCount; i++ {
		preGeneratedMessages[i] = createNewMessage()
	}

	// increase the counter when a missing message was detected
	tangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId message.ID) {
		atomic.AddInt32(&missingMessagesCounter, 1)
	}))

	// decrease the counter when a missing message was received
	tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			atomic.AddInt32(&missingMessagesCounter, -1)
		})
	}))

	// mark the WaitGroup as done if all messages are solid
	tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			if msg.ID() == preGeneratedMessages[messageCount-1].ID() {
				wg.Done()
			}
		})
	}))

	// issue transactions in reverse order
	wg.Add(1)
	for i := len(preGeneratedMessages) - 1; i >= 0; i-- {
		go tangle.AttachMessage(preGeneratedMessages[i])
	}

	// wait for all transactions to become solid
	wg.Wait()

	// make sure that all MessageMissing events also had a corresponding MissingMessageReceived event
	assert.Equal(t, missingMessagesCounter, int32(0))

	// shutdown the tangle
	tangle.Shutdown()
}

func newTestMessage(payloadString string) *message.Message {
	return message.New(message.EmptyID, message.EmptyID, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
