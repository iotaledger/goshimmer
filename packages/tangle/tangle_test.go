package tangle

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/testutil"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkTangle_AttachMessage(b *testing.B) {
	tangle := NewTangle(mapdb.NewMapDB())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	messageBytes := make([]*Message, b.N)
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
	messageTangle := NewTangle(mapdb.NewMapDB())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()

		cachedMsgEvent.Message.Consume(func(msg *Message) {
			fmt.Println("ATTACHED:", msg.ID())
		})
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()

		cachedMsgEvent.Message.Consume(func(msg *Message) {
			fmt.Println("SOLID:", msg.ID())
		})
	}))

	messageTangle.Events.MessageUnsolidifiable.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("UNSOLIDIFIABLE:", messageId)
	}))

	messageTangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.Events.MessageRemoved.Attach(events.NewClosure(func(messageId MessageID) {
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
	missingMessagesMap := make(map[MessageID]bool)
	var missingMessagesMapMutex sync.Mutex
	wg := sync.WaitGroup{}

	// create badger store
	badgerDB, err := testutil.BadgerDB(t)
	require.NoError(t, err)

	// map to keep track of the tips
	tips := datastructure.NewRandomMap()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// setup the message factory
	msgFactory := NewFactory(
		badgerDB,
		[]byte("sequenceKey"),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (MessageID, MessageID) {
			return tips.RandomEntry().(MessageID), tips.RandomEntry().(MessageID)
		}),
	)
	defer msgFactory.Shutdown()

	// create a helper function that creates the messages
	createNewMessage := func() *Message {
		// issue the payload
		msg, err := msgFactory.IssuePayload(NewDataPayload([]byte("0")))
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= widthOfTheTangle {
			if rand.Intn(1000) < 500 {
				tips.Delete(msg.Parent2ID())
			} else {
				tips.Delete(msg.Parent1ID())
			}
		}

		// add current message as a tip
		tips.Set(msg.ID(), msg.ID())

		// return the constructed message
		return msg
	}

	// create the tangle
	tangle := NewTangle(badgerDB)
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	// generate the messages we want to solidify
	preGeneratedMessages := make(map[MessageID]*Message)
	for i := 0; i < messageCount; i++ {
		msg := createNewMessage()

		preGeneratedMessages[msg.ID()] = msg
	}

	fmt.Println("PRE-GENERATING MESSAGES: DONE")

	var receivedTransactionsCounter int32
	tangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		defer cachedMsgEvent.Message.Release()
		defer cachedMsgEvent.MessageMetadata.Release()

		newReceivedTransactionsCounterValue := atomic.AddInt32(&receivedTransactionsCounter, 1)
		if newReceivedTransactionsCounterValue%1000 == 0 {
			fmt.Println("RECEIVED MESSAGES: ", newReceivedTransactionsCounterValue)
			go fmt.Println("MISSING MESSAGES:", len(tangle.MissingMessages()))
		}
	}))

	// increase the counter when a missing message was detected
	tangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
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
	tangle.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()
		cachedMsgEvent.Message.Consume(func(msg *Message) {
			missingMessagesMapMutex.Lock()
			delete(missingMessagesMap, msg.ID())
			missingMessagesMapMutex.Unlock()
		})
	}))

	// mark the WaitGroup as done if all messages are solid
	solidMessageCounter := int32(0)
	tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		defer cachedMsgEvent.MessageMetadata.Release()
		defer cachedMsgEvent.Message.Release()

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
		tangle.AttachMessage(preGeneratedMessages[key.(MessageID)])
	})

	// wait for all transactions to become solid
	wg.Wait()

	// make sure that all MessageMissing events also had a corresponding MissingMessageReceived event
	assert.Equal(t, len(missingMessagesMap), 0)
	assert.Equal(t, len(tangle.MissingMessages()), 0)

	// shutdown the tangle
	tangle.Shutdown()
}
