package tangle

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/testutil"
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
	messageCount := 200000
	widthOfTheTangle := 2500

	// variables required for the test
	missingMessagesMap := make(map[message.ID]bool)
	var missingMessagesMapMutex sync.Mutex
	wg := sync.WaitGroup{}

	// create badger store
	badgerDB, err := testutil.BadgerDB(t)
	require.NoError(t, err)

	// map to keep track of the tips
	tips := datastructure.NewRandomMap()
	tips.Set(message.EmptyID, message.EmptyID)

	// setup the message factory
	msgFactory := messagefactory.New(
		badgerDB,
		[]byte("sequenceKey"),
		identity.GenerateLocalIdentity(),
		messagefactory.TipSelectorFunc(func() (message.ID, message.ID) {
			return tips.RandomEntry().(message.ID), tips.RandomEntry().(message.ID)
		}),
	)
	defer msgFactory.Shutdown()

	// create a helper function that creates the messages
	createNewMessage := func() *message.Message {
		// issue the payload
		msg, err := msgFactory.IssuePayload(payload.NewData([]byte("0")))
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= widthOfTheTangle {
			if rand.Intn(1000) < 500 {
				tips.Delete(msg.BranchID())
			} else {
				tips.Delete(msg.TrunkID())
			}
		}

		// add current message as a tip
		tips.Set(msg.ID(), msg.ID())

		// return the constructed message
		return msg
	}

	// create the tangle
	tangle := New(badgerDB)
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	// generate the messages we want to solidify
	preGeneratedMessages := make(map[message.ID]*message.Message)
	for i := 0; i < messageCount; i++ {
		msg := createNewMessage()

		preGeneratedMessages[msg.ID()] = msg
	}

	fmt.Println("PRE-GENERATING MESSAGES: DONE")

	var receivedTransactionsCounter int32
	tangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		defer cachedMessage.Release()
		defer cachedMessageMetadata.Release()

		newReceivedTransactionsCounterValue := atomic.AddInt32(&receivedTransactionsCounter, 1)
		if newReceivedTransactionsCounterValue%1000 == 0 {
			fmt.Println("RECEIVED MESSAGES: ", newReceivedTransactionsCounterValue)
			go fmt.Println("MISSING MESSAGES:", len(tangle.MissingMessages()))
		}
	}))

	// increase the counter when a missing message was detected
	tangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId message.ID) {
		// attach the message after it has been requested
		go func() {
			time.Sleep(50 * time.Millisecond)

			tangle.AttachMessage(preGeneratedMessages[messageId])
		}()

		missingMessagesMapMutex.Lock()
		missingMessagesMap[messageId] = true
		missingMessagesMapMutex.Unlock()
	}))

	// decrease the counter when a missing message was received
	tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			missingMessagesMapMutex.Lock()
			delete(missingMessagesMap, msg.ID())
			missingMessagesMapMutex.Unlock()
		})
	}))

	// mark the WaitGroup as done if all messages are solid
	solidMessageCounter := int32(0)
	tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		defer cachedMessageMetadata.Release()
		defer cachedMessage.Release()

		// print progress status message
		newSolidCounterValue := atomic.AddInt32(&solidMessageCounter, 1)
		if newSolidCounterValue%1000 == 0 {
			fmt.Println("SOLID MESSAGES: ", newSolidCounterValue)
			go fmt.Println("MISSING MESSAGES:", len(tangle.MissingMessages()))
		}

		// mark WaitGroup as done when we are done solidifying everything
		if newSolidCounterValue == int32(messageCount) {
			fmt.Println("ALL MESSAGES SOLID")

			wg.Done()
		}
	}))

	// issue tips to start solidification
	wg.Add(1)
	tips.ForEach(func(key interface{}, value interface{}) {
		tangle.AttachMessage(preGeneratedMessages[key.(message.ID)])
	})

	// wait for all transactions to become solid
	wg.Wait()

	// make sure that all MessageMissing events also had a corresponding MissingMessageReceived event
	assert.Equal(t, len(missingMessagesMap), 0)
	assert.Equal(t, len(tangle.MissingMessages()), 0)

	// shutdown the tangle
	tangle.Shutdown()
}

func newTestMessage(payloadString string) *message.Message {
	return message.New(message.EmptyID, message.EmptyID, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
