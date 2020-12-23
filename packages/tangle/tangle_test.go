package tangle

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/datastructure/randommap"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkTangle_StoreMessage(b *testing.B) {
	tangle := newTangle(mapdb.NewMapDB())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	messageBytes := make([]*Message, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestDataMessage("some data")
		messageBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.StoreMessage(messageBytes[i])
	}

	tangle.Shutdown()
}

func TestTangle_StoreMessage(t *testing.T) {
	messageTangle := newTangle(mapdb.NewMapDB())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
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

	messageTangle.MessageStore.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.MessageStore.Events.MessageRemoved.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("REMOVED:", messageId)
	}))

	newMessageOne := newTestDataMessage("some data")
	newMessageTwo := newTestDataMessage("some other data")

	messageTangle.StoreMessage(newMessageTwo)

	time.Sleep(7 * time.Second)

	messageTangle.StoreMessage(newMessageOne)

	messageTangle.Shutdown()
}

func TestTangle_MissingMessages(t *testing.T) {
	const (
		messageCount = 20000
		tangleWidth  = 250
		attachDelay  = 5 * time.Millisecond
	)

	// create badger store
	badgerDB, err := testutil.BadgerDB(t)
	require.NoError(t, err)

	// map to keep track of the tips
	tips := randommap.New()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// setup the message factory
	msgFactory := NewMessageFactory(
		badgerDB,
		[]byte("sequenceKey"),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func(count int) []MessageID {
			r := tips.RandomUniqueEntries(count)
			if len(r) == 0 {
				return []MessageID{EmptyMessageID}
			}
			parents := make([]MessageID, len(r))
			for i := range r {
				parents[i] = r[i].(MessageID)
			}
			return parents
		}),
	)
	defer msgFactory.Shutdown()

	// create a helper function that creates the messages
	createNewMessage := func() *Message {
		// issue the payload
		msg, err := msgFactory.IssuePayload(payload.NewGenericDataPayload([]byte("0")))
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			index := rand.Intn(len(msg.StrongParents()))
			tips.Delete(msg.StrongParents()[index])
		}

		// add current message as a tip
		tips.Set(msg.ID(), msg.ID())

		// return the constructed message
		return msg
	}

	// create the tangle
	tangle := newTangle(badgerDB)
	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// generate the messages we want to solidify
	messages := make(map[MessageID]*Message, messageCount)
	for i := 0; i < messageCount; i++ {
		msg := createNewMessage()
		messages[msg.ID()] = msg
	}

	// counter for the different stages
	var (
		attachedMessages int32
		missingMessages  int32
		solidMessages    int32
	)
	tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(event *CachedMessageEvent) {
		defer event.Message.Release()
		defer event.MessageMetadata.Release()

		n := atomic.AddInt32(&attachedMessages, 1)
		t.Logf("attached messages %d/%d", n, messageCount)
	}))

	// increase the counter when a missing message was detected
	tangle.MessageStore.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)

		// attach the message after it has been requested
		go func() {
			time.Sleep(attachDelay)
			tangle.StoreMessage(messages[messageId])
		}()
	}))

	// decrease the counter when a missing message was received
	tangle.MessageStore.Events.MissingMessageReceived.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		defer cachedMsgEvent.Message.Release()
		defer cachedMsgEvent.MessageMetadata.Release()

		n := atomic.AddInt32(&missingMessages, -1)
		t.Logf("missing messages %d", n)
	}))

	tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		defer cachedMsgEvent.MessageMetadata.Release()
		defer cachedMsgEvent.Message.Release()

		atomic.AddInt32(&solidMessages, 1)
	}))

	// issue tips to start solidification
	tips.ForEach(func(key interface{}, _ interface{}) { tangle.StoreMessage(messages[key.(MessageID)]) })

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidMessages) == messageCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, messageCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, messageCount, atomic.LoadInt32(&attachedMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
}

func TestRetrieveAllTips(t *testing.T) {
	messageTangle := newTangle(mapdb.NewMapDB())

	messageA := newTestParentsDataMessage("A", []MessageID{EmptyMessageID}, []MessageID{EmptyMessageID})
	messageB := newTestParentsDataMessage("B", []MessageID{messageA.ID()}, []MessageID{EmptyMessageID})
	messageC := newTestParentsDataMessage("C", []MessageID{messageA.ID()}, []MessageID{EmptyMessageID})

	var wg sync.WaitGroup

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.Message.Release()
		cachedMsgEvent.MessageMetadata.Release()
		wg.Done()
	}))

	wg.Add(3)
	messageTangle.StoreMessage(messageA)
	messageTangle.StoreMessage(messageB)
	messageTangle.StoreMessage(messageC)

	wg.Wait()

	allTips := messageTangle.RetrieveAllTips()

	assert.Equal(t, 2, len(allTips))

	messageTangle.Shutdown()
}

func newTangle(store kvstore.KVStore) *Tangle {
	tangle := New(store)

	// Attach solidification
	// TODO: the solidification will be attached to other event in the future refactoring
	// MessageStored event will trigger timestamp check
	tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		tangle.SolidifyMessage(cachedMsgEvent.Message, cachedMsgEvent.MessageMetadata)
	}))

	return tangle
}
