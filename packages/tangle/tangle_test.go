package tangle

import (
	"context"
	"crypto"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/randommap"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/testutil"
	"github.com/iotaledger/hive.go/workerpool"
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

func TestTangle_InvalidParentsAgeMessage(t *testing.T) {
	messageTangle := newTangle(mapdb.NewMapDB())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	var (
		storedMessages  int32
		solidMessages   int32
		invalidMessages int32
	)

	newOldParentsMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now().Add(15*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Old")), 0, ed25519.Signature{})
	}
	newYoungParentsMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now().Add(-15*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Young")), 0, ed25519.Signature{})
	}
	newValidMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Valid")), 0, ed25519.Signature{})
	}

	messageTangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()

		cachedMsgEvent.Message.Consume(func(msg *Message) {
			fmt.Println("STORED:", msg.ID())
		})
		atomic.AddInt32(&storedMessages, 1)
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()

		cachedMsgEvent.Message.Consume(func(msg *Message) {
			fmt.Println("SOLID:", msg.ID())
		})
		atomic.AddInt32(&solidMessages, 1)
	}))

	messageTangle.Events.MessageInvalid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.MessageMetadata.Release()

		cachedMsgEvent.Message.Consume(func(msg *Message) {
			fmt.Println("INVALID:", msg.ID())
		})
		atomic.AddInt32(&invalidMessages, 1)
	}))

	messageA := newTestDataMessage("some data")
	messageB := newTestDataMessage("some data")
	messageC := newValidMessage([]MessageID{messageA.ID(), messageB.ID()})
	messageOldParents := newOldParentsMessage([]MessageID{messageA.ID(), messageB.ID()})
	messageYoungParents := newYoungParentsMessage([]MessageID{messageA.ID(), messageB.ID()})

	messageTangle.StoreMessage(messageA)
	messageTangle.StoreMessage(messageB)

	time.Sleep(7 * time.Second)

	messageTangle.StoreMessage(messageC)
	messageTangle.StoreMessage(messageOldParents)
	messageTangle.StoreMessage(messageYoungParents)

	// wait for all messages to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&storedMessages) == 5 }, 1*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, 3, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, 2, atomic.LoadInt32(&invalidMessages))

	messageTangle.Shutdown()
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
			fmt.Println("STORED:", msg.ID())
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
		storeDelay   = 5 * time.Millisecond
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
		storedMessages  int32
		missingMessages int32
		solidMessages   int32
	)
	tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(event *CachedMessageEvent) {
		defer event.Message.Release()
		defer event.MessageMetadata.Release()

		n := atomic.AddInt32(&storedMessages, 1)
		t.Logf("stored messages %d/%d", n, messageCount)
	}))

	// increase the counter when a missing message was detected
	tangle.MessageStore.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)

		// store the message after it has been requested
		go func() {
			time.Sleep(storeDelay)
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
	assert.EqualValues(t, messageCount, atomic.LoadInt32(&storedMessages))
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

func TestTangle_FilterStoreSolidify(t *testing.T) {
	const (
		testNetwork = "udp"
		testPort    = 8000
		targetPOW   = 2

		solidMsgCount   = 20000
		invalidMsgCount = 10
		tangleWidth     = 250
		networkDelay    = 5 * time.Millisecond
	)

	var (
		totalMsgCount = solidMsgCount + invalidMsgCount
		testWorker    = pow.New(crypto.BLAKE2b_512, 1)
		// same as gossip manager
		messageWorkerCount     = runtime.GOMAXPROCS(0) * 4
		messageWorkerQueueSize = 1000
	)
	// create badger store
	badgerDB, err := testutil.BadgerDB(t)
	require.NoError(t, err)

	// map to keep track of the tips
	tips := randommap.New()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// create local peer
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testPort)
	localIdentity := identity.GenerateLocalIdentity()
	localPeer := peer.NewPeer(localIdentity.Identity, net.IPv4zero, services)

	// setup the message parser
	msgParser := NewMessageParser()
	msgParser.AddBytesFilter(NewPowFilter(testWorker, targetPOW))
	msgParser.Setup()

	// setup the message factory
	msgFactory := NewMessageFactory(
		badgerDB,
		[]byte("sequenceKey"),
		localIdentity,
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

	// PoW workers
	msgFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return testWorker.Mine(context.Background(), content, targetPOW)
	}))
	defer msgFactory.Shutdown()

	// create a helper function that creates the messages
	createNewMessage := func(invalidTS bool) *Message {
		var msg *Message
		var err error

		// issue the payload
		if invalidTS {
			msg, err = msgFactory.issueInvalidTsPayload(payload.NewGenericDataPayload([]byte("0")))
		} else {
			msg, err = msgFactory.IssuePayload(payload.NewGenericDataPayload([]byte("0")))
		}
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			index := rand.Intn(len(msg.StrongParents()))
			tips.Delete(msg.StrongParents()[index])
		}

		// add current message as a tip
		// only valid message will be in the tip set
		if !invalidTS {
			tips.Set(msg.ID(), msg.ID())
		}
		// return the constructed message
		return msg
	}

	// create inboxWP to act as the gossip layer
	inboxWP := workerpool.New(func(task workerpool.Task) {

		time.Sleep(networkDelay)
		msgParser.Parse(task.Param(0).([]byte), task.Param(1).(*peer.Peer))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))
	inboxWP.Start()
	defer inboxWP.Stop()

	// create the tangle
	tangle := newTangle(badgerDB)
	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// generate the messages we want to solidify
	messages := make(map[MessageID]*Message, solidMsgCount)
	for i := 0; i < solidMsgCount; i++ {
		msg := createNewMessage(false)
		messages[msg.ID()] = msg
	}

	// generate the invalid timestamp messages
	invalidmsgs := make(map[MessageID]*Message, invalidMsgCount)
	for i := 0; i < invalidMsgCount; i++ {
		msg := createNewMessage(true)
		invalidmsgs[msg.ID()] = msg
	}

	// counter for the different stages
	var (
		parsedMessages   int32
		storedMessages   int32
		missingMessages  int32
		solidMessages    int32
		invalidMessages  int32
		rejectedMessages int32
	)

	// filter rejected events
	msgParser.Events.MessageRejected.Attach(events.NewClosure(func(msgRejectedEvent *MessageRejectedEvent, _ error) {
		n := atomic.AddInt32(&rejectedMessages, 1)
		t.Logf("rejected by message filter messages %d/%d", n, totalMsgCount)
	}))

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		n := atomic.AddInt32(&parsedMessages, 1)
		t.Logf("parsed messages %d/%d", n, totalMsgCount)

		tangle.StoreMessage(msgParsedEvent.Message)
	}))

	// message invalid events
	invalidSeen := make(map[MessageID]bool)
	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		defer cachedMsgEvent.MessageMetadata.Release()
		defer cachedMsgEvent.Message.Release()

		msg := cachedMsgEvent.Message.Unwrap()
		_, ok := invalidSeen[msg.ID()]
		if ok {
			// avoid increasing the counter for seen invalid messages
			// this check can be removed when message store cleaner is implemented.
			return
		}
		invalidSeen[msg.ID()] = true

		n := atomic.AddInt32(&invalidMessages, 1)
		t.Logf("invalid messages %d/%d", n, totalMsgCount)
	}))

	tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(event *CachedMessageEvent) {
		defer event.Message.Release()
		defer event.MessageMetadata.Release()

		n := atomic.AddInt32(&storedMessages, 1)
		t.Logf("stored messages %d/%d", n, totalMsgCount)
	}))

	// increase the counter when a missing message was detected
	tangle.MessageStore.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)

		// push the message into the gossip inboxWP
		inboxWP.TrySubmit(messages[messageId].Bytes(), localPeer)
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
	tips.ForEach(func(key interface{}, _ interface{}) { inboxWP.TrySubmit(messages[key.(MessageID)].Bytes(), localPeer) })
	// incoming invalid messages
	for _, msg := range invalidmsgs {
		inboxWP.TrySubmit(msg.Bytes(), localPeer)
	}

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidMessages) == solidMsgCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, totalMsgCount, atomic.LoadInt32(&storedMessages))
	assert.EqualValues(t, totalMsgCount, atomic.LoadInt32(&parsedMessages))
	assert.EqualValues(t, invalidMsgCount, atomic.LoadInt32(&invalidMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
}

func newTangle(store kvstore.KVStore) *Tangle {
	tangle := New(store)

	// Attach solidification
	// TODO: the solidification will be attached to other event in the future refactoring
	tangle.MessageStore.Events.MessageStored.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		tangle.SolidifyMessage(cachedMsgEvent.Message, cachedMsgEvent.MessageMetadata)
	}))

	return tangle
}

// IssueInvalidTsPayload creates a new message including sequence number and tip selection and returns it.
func (f *MessageFactory) issueInvalidTsPayload(p payload.Payload, t ...*Tangle) (*Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	f.issuanceMutex.Lock()
	defer f.issuanceMutex.Unlock()
	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = fmt.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	strongParents := f.selector.Tips(2)
	weakParents := make([]MessageID, 0)
	issuingTime := time.Now().Add(15 * time.Minute)
	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	nonce, err := f.doPOW(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p)
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature := f.sign(strongParents, weakParents, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)

	msg := NewMessage(
		strongParents,
		weakParents,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	return msg, nil
}
