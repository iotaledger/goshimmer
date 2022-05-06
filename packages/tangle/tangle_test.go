package tangle

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/randommap"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/testutil"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/consensus/otv"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func BenchmarkVerifySignature(b *testing.B) {
	tangle := NewTestTangle()

	pool, _ := ants.NewPool(80, ants.WithNonblocking(false))

	factory := NewMessageFactory(tangle, TipSelectorFunc(func(p payload.Payload, countStrongParents int) (parents MessageIDs, err error) {
		return NewMessageIDs(EmptyMessageID), nil
	}), emptyLikeReferences)

	messages := make([]*Message, b.N)
	for i := 0; i < b.N; i++ {
		msg, err := factory.IssuePayload(payload.NewGenericDataPayload([]byte("some data")))
		require.NoError(b, err)
		messages[i] = msg
		messages[i].Bytes()
	}
	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		currentIndex := i
		if err := pool.Submit(func() {
			messages[currentIndex].VerifySignature()
			wg.Done()
		}); err != nil {
			b.Error(err)
			return
		}
	}

	wg.Wait()
}

func BenchmarkTangle_StoreMessage(b *testing.B) {
	tangle := NewTestTangle()
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
	messageTangle := NewTestTangle()
	messageTangle.Storage.Setup()
	messageTangle.Solidifier.Setup()
	defer messageTangle.Shutdown()

	var storedMessages, solidMessages, invalidMessages int32

	newOldParentsMessage := func(strongParents MessageIDs) *Message {
		message, err := NewMessage(emptyLikeReferencesFromStrongParents(strongParents), time.Now().Add(maxParentsTimeDifference+5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Old")), 0, ed25519.Signature{})
		assert.NoError(t, err)
		return message
	}
	newYoungParentsMessage := func(strongParents MessageIDs) *Message {
		message, err := NewMessage(emptyLikeReferencesFromStrongParents(strongParents), time.Now().Add(-maxParentsTimeDifference-5*time.Minute), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Young")), 0, ed25519.Signature{})
		assert.NoError(t, err)
		return message
	}
	newValidMessage := func(strongParents MessageIDs) *Message {
		message, err := NewMessage(emptyLikeReferencesFromStrongParents(strongParents), time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("Valid")), 0, ed25519.Signature{})
		assert.NoError(t, err)
		return message
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

	messageTangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageInvalidEvent *MessageInvalidEvent) {
		fmt.Println("INVALID:", messageInvalidEvent.MessageID)
		atomic.AddInt32(&invalidMessages, 1)
	}))

	messageA := newTestDataMessage("some data")
	messageB := newTestDataMessage("some data1")
	messageC := newValidMessage(NewMessageIDs(messageA.ID(), messageB.ID()))
	messageOldParents := newOldParentsMessage(NewMessageIDs(messageA.ID(), messageB.ID()))
	messageYoungParents := newYoungParentsMessage(NewMessageIDs(messageA.ID(), messageB.ID()))

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
	messageTangle := NewTestTangle()
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
		messageCount = 2000
		tangleWidth  = 250
		storeDelay   = 5 * time.Millisecond
	)

	// create rocksdb store
	rocksdb, err := testutil.RocksDB(t)
	require.NoError(t, err)

	// create the tangle
	tangle := NewTestTangle(Store(rocksdb), Identity(selfLocalIdentity))
	tangle.OTVConsensusManager = NewOTVConsensusManager(otv.NewOnTangleVoting(tangle.LedgerState.BranchDAG, tangle.ApprovalWeightManager.WeightOfBranch))

	defer tangle.Shutdown()
	require.NoError(t, tangle.Prune())

	// map to keep track of the tips
	tips := randommap.New[MessageID, MessageID]()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// setup the message factory
	tangle.MessageFactory = NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsMessageIDs MessageIDs, err error) {
			r := tips.RandomUniqueEntries(countParents)
			if len(r) == 0 {
				return NewMessageIDs(EmptyMessageID), nil
			}
			parents := NewMessageIDs()
			for _, tip := range r {
				parents.Add(tip)
			}
			return parents, nil
		}),
		emptyLikeReferences,
	)

	// create a helper function that creates the messages
	createNewMessage := func() *Message {
		// issue the payload
		msg, err := tangle.MessageFactory.IssuePayload(payload.NewGenericDataPayload([]byte("")))
		require.NoError(t, err)

		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			tips.Delete(msg.ParentsByType(StrongParentType).First())
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

	// manually set up Tangle flow as far as we need it
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()

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
	tips.ForEach(func(key MessageID, _ MessageID) { tangle.Storage.StoreMessage(messages[key]) })

	// wait for all transactions to become solid
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidMessages) == messageCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValues(t, messageCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, messageCount, atomic.LoadInt32(&storedMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
}

func TestRetrieveAllTips(t *testing.T) {
	messageTangle := NewTestTangle(Identity(selfLocalIdentity))
	messageTangle.Setup()
	defer messageTangle.Shutdown()

	messageA := newTestParentsDataMessage("A", ParentMessageIDs{
		StrongParentType: NewMessageIDs(EmptyMessageID),
	})
	messageB := newTestParentsDataMessage("B", ParentMessageIDs{
		StrongParentType: NewMessageIDs(messageA.ID()),
	})
	messageC := newTestParentsDataMessage("C", ParentMessageIDs{
		StrongParentType: NewMessageIDs(messageA.ID()),
	})

	var wg sync.WaitGroup

	messageTangle.Dispatcher.Events.MessageDispatched.Attach(events.NewClosure(func(MessageID) {
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

		solidMsgCount   = 2000
		invalidMsgCount = 10
		tangleWidth     = 250
		networkDelay    = 5 * time.Millisecond
	)

	var (
		totalMsgCount = solidMsgCount + invalidMsgCount
		testWorker    = pow.New(1)
		// same as gossip manager
		messageWorkerCount     = runtime.GOMAXPROCS(0) * 4
		messageWorkerQueueSize = 1000
	)
	// create rocksdb store
	rocksdb, err := testutil.RocksDB(t)
	require.NoError(t, err)

	// map to keep track of the tips
	tips := randommap.New[MessageID, MessageID]()
	tips.Set(EmptyMessageID, EmptyMessageID)

	// create the tangle
	tangle := NewTestTangle(Store(rocksdb), Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	// create local peer
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testPort)
	localIdentity := tangle.Options.Identity
	localPeer := peer.NewPeer(localIdentity.Identity, net.IPv4zero, services)

	// set up the message factory
	tangle.MessageFactory = NewMessageFactory(
		tangle,
		TipSelectorFunc(func(p payload.Payload, countParents int) (parentsMessageIDs MessageIDs, err error) {
			r := tips.RandomUniqueEntries(countParents)
			if len(r) == 0 {
				return NewMessageIDs(EmptyMessageID), nil
			}

			parents := NewMessageIDs()
			for _, tip := range r {
				parents.Add(tip)
			}
			return parents, nil
		}),
		emptyLikeReferences,
	)

	// PoW workers
	tangle.MessageFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return testWorker.Mine(context.Background(), content, targetPOW)
	}))
	tangle.MessageFactory.SetTimeout(powTimeout)
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
		if !invalidTS {
			if tips.Size() >= tangleWidth {
				tips.Delete(msg.ParentsByType(StrongParentType).First())
			}
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
	inboxWP := workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		time.Sleep(networkDelay)
		tangle.ProcessGossipMessage(task.Param(0).([]byte), task.Param(1).(*peer.Peer))

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))
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

	// counter for the different stages
	var (
		parsedMessages     int32
		storedMessages     int32
		missingMessages    int32
		solidMessages      int32
		scheduledMessages  int32
		bookedMessages     int32
		dispatchedMessages int32
		awMessages         int32
		invalidMessages    int32
		rejectedMessages   int32
	)

	tangle.Parser.Events.BytesRejected.AttachAfter(events.NewClosure(func(e *BytesRejectedEvent, err error) {
		t.Logf("rejected bytes %v - %s", e.Bytes, err)
	}))

	// filter rejected events
	tangle.Parser.Events.MessageRejected.AttachAfter(events.NewClosure(func(msgRejectedEvent *MessageRejectedEvent, err error) {
		n := atomic.AddInt32(&rejectedMessages, 1)
		t.Logf("rejected by message filter messages %d/%d - %s %s", n, totalMsgCount, msgRejectedEvent.Message.ID(), err)
	}))

	tangle.Parser.Events.MessageParsed.AttachAfter(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		n := atomic.AddInt32(&parsedMessages, 1)
		t.Logf("parsed messages %d/%d - %s", n, totalMsgCount, msgParsedEvent.Message.ID())
	}))

	// message invalid events
	tangle.Events.MessageInvalid.AttachAfter(events.NewClosure(func(messageInvalidEvent *MessageInvalidEvent) {
		n := atomic.AddInt32(&invalidMessages, 1)
		t.Logf("invalid messages %d/%d - %s", n, totalMsgCount, messageInvalidEvent.MessageID)
	}))

	tangle.Storage.Events.MessageStored.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&storedMessages, 1)
		t.Logf("stored messages %d/%d - %s", n, totalMsgCount, messageID)
	}))

	// increase the counter when a missing message was detected
	tangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(func(messageId MessageID) {
		atomic.AddInt32(&missingMessages, 1)

		// push the message into the gossip inboxWP
		inboxWP.TrySubmit(messages[messageId].Bytes(), localPeer)
	}))

	// decrease the counter when a missing message was received
	tangle.Storage.Events.MissingMessageStored.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&missingMessages, -1)
		t.Logf("missing messages %d - %s", n, messageID)
	}))

	tangle.Solidifier.Events.MessageSolid.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&solidMessages, 1)
		t.Logf("solid messages %d/%d - %s", n, totalMsgCount, messageID)
	}))

	tangle.Scheduler.Events.MessageScheduled.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&scheduledMessages, 1)
		t.Logf("scheduled messages %d/%d - %s", n, totalMsgCount, messageID)
	}))

	tangle.Booker.Events.MessageBooked.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&bookedMessages, 1)
		t.Logf("booked messages %d/%d - %s", n, totalMsgCount, messageID)
	}))

	tangle.Dispatcher.Events.MessageDispatched.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&dispatchedMessages, 1)
		t.Logf("dispatched messages %d/%d", n, totalMsgCount)
	}))
	tangle.ApprovalWeightManager.Events.MessageProcessed.AttachAfter(events.NewClosure(func(messageID MessageID) {
		n := atomic.AddInt32(&awMessages, 1)
		t.Logf("approval weight processed messages %d/%d", n, totalMsgCount)
	}))

	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Logf("Error %s", err)
	}))

	// setup data flow
	tangle.Setup()

	// issue tips to start solidification
	tips.ForEach(func(key MessageID, _ MessageID) {
		if key == EmptyMessageID {
			return
		}
		inboxWP.TrySubmit(messages[key].Bytes(), localPeer)
	})
	// incoming invalid messages
	for _, msg := range invalidmsgs {
		inboxWP.TrySubmit(msg.Bytes(), localPeer)
	}

	// wait for all messages to be scheduled
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&scheduledMessages) == solidMsgCount }, 5*time.Minute, 100*time.Millisecond)

	assert.EqualValuesf(t, totalMsgCount, atomic.LoadInt32(&parsedMessages), "parsed messages does not match")
	assert.EqualValuesf(t, totalMsgCount, atomic.LoadInt32(&storedMessages), "stored messages does not match")
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&solidMessages))
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&scheduledMessages))
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&bookedMessages))
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&dispatchedMessages))
	assert.EqualValues(t, solidMsgCount, atomic.LoadInt32(&awMessages))
	// messages with invalid timestamp are not forwarded from the timestamp filter, thus there are 0.
	assert.EqualValues(t, invalidMsgCount, atomic.LoadInt32(&invalidMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&rejectedMessages))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingMessages))
}

// IssueInvalidTsPayload creates a new message including sequence number and tip selection and returns it.
func (f *MessageFactory) issueInvalidTsPayload(p payload.Payload, _ ...*Tangle) (*Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = fmt.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	parents, err := f.selector.Tips(p, 2)
	if err != nil {
		err = fmt.Errorf("could not select tips: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	issuingTime := time.Now().Add(maxParentsTimeDifference + 5*time.Minute)
	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	nonce, err := f.doPOW(emptyLikeReferencesFromStrongParents(parents), issuingTime, issuerPublicKey, sequenceNumber, p)
	if err != nil {
		err = fmt.Errorf("pow failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature, err := f.sign(emptyLikeReferencesFromStrongParents(parents), issuingTime, issuerPublicKey, sequenceNumber, p, nonce)
	if err != nil {
		err = fmt.Errorf("signing failed failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	msg, err := NewMessage(
		emptyLikeReferencesFromStrongParents(parents),
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	if err != nil {
		err = fmt.Errorf("problem with message syntax: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	return msg, nil
}
