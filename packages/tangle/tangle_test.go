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
	"github.com/iotaledger/hive.go/testutil"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkTangle_StoreMessage(b *testing.B) {
	tangle := New()
	defer tangle.Shutdown()
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
		tangle.Storage.StoreMessage(messageBytes[i])
	}
}

func TestTangle_InvalidParentsAgeMessage(t *testing.T) {
	messageTangle := New()
	messageTangle.Setup()
	defer messageTangle.Shutdown()
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	var storedMessages, solidMessages, invalidMessages int32

	newOldParentsMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now().Add(maxParentsTimeDifference+5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Old")), 0, ed25519.Signature{})
	}
	newYoungParentsMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now().Add(-maxParentsTimeDifference-5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Young")), 0, ed25519.Signature{})
	}
	newValidMessage := func(strongParents []MessageID) *Message {
		return NewMessage(strongParents, []MessageID{}, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Valid")), 0, ed25519.Signature{})
	}

	var wg sync.WaitGroup
	messageTangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("STORED:", messageID)
		atomic.AddInt32(&storedMessages, 1)
		wg.Done()
	}))

	messageTangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("SOLID:", messageID)
		atomic.AddInt32(&solidMessages, 1)
	}))

	messageTangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("INVALID:", messageID)
		atomic.AddInt32(&invalidMessages, 1)
	}))

	messageA := newTestDataMessage("some data")
	messageB := newTestDataMessage("some data1")
	messageC := newValidMessage([]MessageID{messageA.ID(), messageB.ID()})
	messageOldParents := newOldParentsMessage([]MessageID{messageA.ID(), messageB.ID()})
	messageYoungParents := newYoungParentsMessage([]MessageID{messageA.ID(), messageB.ID()})

	wg.Add(5)
	messageTangle.Storage.StoreMessage(messageA)
	messageTangle.Storage.StoreMessage(messageB)
	messageTangle.Storage.StoreMessage(messageC)
	messageTangle.Storage.StoreMessage(messageOldParents)
	messageTangle.Storage.StoreMessage(messageYoungParents)

	// wait for all messages to become solid
	wg.Wait()

	assert.EqualValues(t, 5, atomic.LoadInt32(&storedMessages))
	assert.EqualValues(t, 3, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, 2, atomic.LoadInt32(&invalidMessages))
}

func TestTangle_StoreMessage(t *testing.T) {
	messageTangle := New()
	defer messageTangle.Shutdown()
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("STORED:", messageID)
	}))

	messageTangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("SOLID:", messageID)
	}))

	messageTangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.Storage.Events.MessageRemoved.Attach(events.NewClosure(func(messageId MessageID) {
		fmt.Println("REMOVED:", messageId)
	}))

	newMessageOne := newTestDataMessage("some data")
	newMessageTwo := newTestDataMessage("some other data")

	messageTangle.Storage.StoreMessage(newMessageTwo)

	time.Sleep(7 * time.Second)

	messageTangle.Storage.StoreMessage(newMessageOne)
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

	// create the tangle
	tangle := New(Store(badgerDB))
	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// map to keep track of the tips
	tips := randommap.New()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// setup the message factory
	tangle.MessageFactory = NewMessageFactory(
		tangle,
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

	// create a helper function that creates the messages
	createNewMessage := func() *Message {
		// issue the payload
		msg, err := tangle.MessageFactory.IssuePayload(payload.NewGenericDataPayload([]byte("")))
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

	// generate the messages we want to solidify
	messages := make(map[MessageID]*Message, messageCount)
	for i := 0; i < messageCount; i++ {
		msg := createNewMessage()
		messages[msg.ID()] = msg
	}

	tangle.Setup()

	// counter for the different stages
	var (
		storedMessages  int32
		missingMessages int32
		solidMessages   int32
	)
	tangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&storedMessages, 1)
		t.Logf("stored messages %d/%d", n, messageCount)
	}))

	// increase the counter when a missing message was detected
	tangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)
		// store the message after it has been requested
		go func() {
			time.Sleep(storeDelay)
			tangle.Storage.StoreMessage(messages[messageId])
		}()
	}))

	// decrease the counter when a missing message was received
	tangle.Storage.Events.MissingMessageStored.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&missingMessages, -1)
		t.Logf("missing messages %d", n)
	}))

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&solidMessages, 1)
		t.Logf("solid messages %d/%d", n, messageCount)
	}))

	// issue tips to start solidification
	tips.ForEach(func(key interface{}, _ interface{}) { tangle.Storage.StoreMessage(messages[key.(MessageID)]) })

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidMessages) == messageCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, messageCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, messageCount, atomic.LoadInt32(&storedMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
}

func TestRetrieveAllTips(t *testing.T) {
	messageTangle := New()
	messageTangle.Setup()
	defer messageTangle.Shutdown()

	messageA := newTestParentsDataMessage("A", []MessageID{EmptyMessageID}, []MessageID{EmptyMessageID})
	messageB := newTestParentsDataMessage("B", []MessageID{messageA.ID()}, []MessageID{EmptyMessageID})
	messageC := newTestParentsDataMessage("C", []MessageID{messageA.ID()}, []MessageID{EmptyMessageID})

	var wg sync.WaitGroup

	messageTangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(MessageID) {
		wg.Done()
	}))

	wg.Add(3)
	messageTangle.Storage.StoreMessage(messageA)
	messageTangle.Storage.StoreMessage(messageB)
	messageTangle.Storage.StoreMessage(messageC)

	wg.Wait()

	allTips := messageTangle.Storage.RetrieveAllTips()

	assert.Equal(t, 2, len(allTips))
}

func TestTangle_Flow(t *testing.T) {
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

	// create the tangle
	tangle := New(Store(badgerDB))
	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// create local peer
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testPort)
	localIdentity := tangle.Options.Identity
	localPeer := peer.NewPeer(localIdentity.Identity, net.IPv4zero, services)

	// setup the message factory
	tangle.MessageFactory = NewMessageFactory(
		tangle,
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
	tangle.MessageFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return testWorker.Mine(context.Background(), content, targetPOW)
	}))

	// create a helper function that creates the messages
	createNewMessage := func(invalidTS bool) *Message {
		var msg *Message
		var err error

		// issue the payload
		if invalidTS {
			msg, err = tangle.MessageFactory.issueInvalidTsPayload(payload.NewGenericDataPayload([]byte("")))
		} else {
			msg, err = tangle.MessageFactory.IssuePayload(payload.NewGenericDataPayload([]byte("")))
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

	// setup the message parser
	tangle.Parser.AddBytesFilter(NewPowFilter(testWorker, targetPOW))

	// create inboxWP to act as the gossip layer
	inboxWP := workerpool.New(func(task workerpool.Task) {
		time.Sleep(networkDelay)
		tangle.ProcessGossipMessage(task.Param(0).([]byte), task.Param(1).(*peer.Peer))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))
	inboxWP.Start()
	defer inboxWP.Stop()

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

	// setup data flow
	tangle.Setup()

	// counter for the different stages
	var (
		parsedMessages    int32
		storedMessages    int32
		missingMessages   int32
		solidMessages     int32
		scheduledMessages int32
		invalidMessages   int32
		rejectedMessages  int32
	)

	// filter rejected events
	tangle.Parser.Events.MessageRejected.Attach(events.NewClosure(func(msgRejectedEvent *MessageRejectedEvent, _ error) {
		n := atomic.AddInt32(&rejectedMessages, 1)
		t.Logf("rejected by message filter messages %d/%d", n, totalMsgCount)
	}))

	tangle.Parser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		n := atomic.AddInt32(&parsedMessages, 1)
		t.Logf("parsed messages %d/%d", n, totalMsgCount)

		tangle.Storage.StoreMessage(msgParsedEvent.Message)
	}))

	// message invalid events
	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		tangle.Storage.DeleteMessage(messageID)

		n := atomic.AddInt32(&invalidMessages, 1)
		t.Logf("invalid messages %d/%d", n, totalMsgCount)
	}))

	tangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&storedMessages, 1)
		t.Logf("stored messages %d/%d", n, totalMsgCount)
	}))

	// increase the counter when a missing message was detected
	tangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)

		// push the message into the gossip inboxWP
		inboxWP.TrySubmit(messages[messageId].Bytes(), localPeer)
	}))

	// decrease the counter when a missing message was received
	tangle.Storage.Events.MissingMessageStored.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&missingMessages, -1)
		t.Logf("missing messages %d", n)
	}))

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(MessageID) {
		n := atomic.AddInt32(&solidMessages, 1)
		t.Logf("solid messages %d/%d", n, totalMsgCount)
	}))

	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		// Bypassing the messageBooked event
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.Booker.Events.MessageBooked.Trigger(messageID)
		})

		n := atomic.AddInt32(&scheduledMessages, 1)
		t.Logf("scheduled messages %d/%d", n, totalMsgCount)
	}))

	// issue tips to start solidification
	tips.ForEach(func(key interface{}, _ interface{}) {
		inboxWP.TrySubmit(messages[key.(MessageID)].Bytes(), localPeer)
	})
	// incoming invalid messages
	for _, msg := range invalidmsgs {
		inboxWP.TrySubmit(msg.Bytes(), localPeer)
	}

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&scheduledMessages) == solidMsgCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&scheduledMessages))
	assert.EqualValues(t, totalMsgCount, atomic.LoadInt32(&storedMessages))
	assert.EqualValues(t, totalMsgCount, atomic.LoadInt32(&parsedMessages))
	assert.EqualValues(t, invalidMsgCount, atomic.LoadInt32(&invalidMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
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
	issuingTime := time.Now().Add(maxParentsTimeDifference + 5*time.Minute)
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
