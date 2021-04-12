package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

// region Scheduler_test /////////////////////////////////////////////////////////////////////////////////////////////

var (
	selfLocalIdentity   = identity.GenerateLocalIdentity()
	selfNode            = identity.New(selfLocalIdentity.PublicKey())
	peerNode            = identity.GenerateIdentity()
	otherNode           = identity.GenerateIdentity()
	testSchedulerParams = SchedulerParams{
		RateSetterInitial:           MaxMessageSize * 200,
		RateSetterBeta:              1,
		AccessManaRetrieveFunc:      getAccessMana,
		TotalAccessManaRetrieveFunc: getTotalAccessMana}
)

func TestScheduler_StartStop(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()
	time.Sleep(10 * time.Millisecond)
}

func TestScheduler_Submit(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()

	msg := newMessage(selfNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	tangle.Scheduler.Submit(msg.ID())
	time.Sleep(100 * time.Millisecond)
}

func TestScheduler_Discarded(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()

	messageDiscarded := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageDiscarded.Attach(events.NewClosure(func(id MessageID) { messageDiscarded <- id }))

	// this node has no mana so the message will be discarded
	msg := newMessage(otherNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	tangle.Scheduler.Submit(msg.ID())

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageDiscarded:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Schedule(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()

	messageScheduled := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	// create a new message from a different node
	msg := newMessage(peerNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	tangle.Scheduler.Submit(msg.ID())
	tangle.Scheduler.Ready(msg.ID())

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageScheduled:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Time(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()

	messageScheduled := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	future := newMessage(peerNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	tangle.Storage.StoreMessage(future)
	tangle.Scheduler.Submit(future.ID())

	now := newMessage(peerNode.PublicKey())
	tangle.Storage.StoreMessage(now)
	tangle.Scheduler.Submit(now.ID())

	tangle.Scheduler.Ready(future.ID())
	tangle.Scheduler.Ready(now.ID())

	done := make(chan struct{})
	var scheduledIDs []MessageID
	go func() {
		defer close(done)
		timer := time.NewTimer(time.Until(future.IssuingTime()) + 100*time.Millisecond)
		for {
			select {
			case <-timer.C:
				return
			case id := <-messageScheduled:
				tangle.Storage.Message(id).Consume(func(msg *Message) {
					assert.Truef(t, time.Now().After(msg.IssuingTime()), "scheduled too early: %s", time.Until(msg.IssuingTime()))
					scheduledIDs = append(scheduledIDs, id)
				})
			}
		}
	}()

	<-done
	assert.Equal(t, []MessageID{now.ID(), future.ID()}, scheduledIDs)
}

func TestScheduler_Issue(t *testing.T) {
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()
	tangle.Setup()

	const numMessages = 5
	ids := make([]MessageID, numMessages)
	for i := range ids {
		msg := newMessage(selfNode.PublicKey())
		tangle.Storage.StoreMessage(msg)
		ids[i] = msg.ID()
	}

	messageScheduled := make(chan MessageID, numMessages)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	var scheduledIDs []MessageID
	assert.Eventually(t, func() bool {
		select {
		case id := <-messageScheduled:
			scheduledIDs = append(scheduledIDs, id)
			return len(scheduledIDs) == len(ids)
		default:
			return false
		}
	}, 10*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, ids, scheduledIDs)
}

func TestSchedulerFlow(t *testing.T) {
	// create Scheduler dependencies
	// create the tangle
	tangle := New(Identity(selfLocalIdentity), SchedulerConfig(testSchedulerParams))
	defer tangle.Shutdown()

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()

	// testing desired scheduled order: A - B - D - C  (B - A - D - C is equivalent)
	messages := make(map[string]*Message)
	messages["A"] = newMessage(selfNode.PublicKey())
	messages["B"] = newMessage(peerNode.PublicKey())

	// set C to have a timestamp in the future
	msgC := newMessage(selfNode.PublicKey())
	msgC.strongParents = []MessageID{messages["A"].ID(), messages["B"].ID()}
	msgC.issuingTime = time.Now().Add(5 * time.Second)
	messages["C"] = msgC

	msgD := newMessage(peerNode.PublicKey())
	msgD.strongParents = []MessageID{messages["A"].ID(), messages["B"].ID()}
	messages["D"] = msgD

	msgE := newMessage(selfNode.PublicKey())
	msgE.strongParents = []MessageID{messages["A"].ID(), messages["B"].ID()}
	msgE.issuingTime = time.Now().Add(3 * time.Second)
	messages["E"] = msgE

	messageScheduled := make(chan MessageID, len(messages))
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	// Bypass the Booker
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.Booker.Events.MessageBooked.Trigger(messageID)
			tangle.ConsensusManager.Events.MessageOpinionFormed.Trigger(messageID)
		})
	}))

	for _, message := range messages {
		tangle.Storage.StoreMessage(message)
	}

	var scheduledIDs []MessageID
	assert.Eventually(t, func() bool {
		select {
		case id := <-messageScheduled:
			scheduledIDs = append(scheduledIDs, id)
			return len(scheduledIDs) == len(messages)
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond)
}

func getAccessMana(nodeID identity.ID) float64 {
	if nodeID == otherNode.ID() {
		return 0
	}
	return 8
}

func getTotalAccessMana() float64 {
	return 16
}

func newMessage(issuerPublicKey ed25519.PublicKey) *Message {
	return NewMessage(
		[]MessageID{EmptyMessageID},
		[]MessageID{EmptyMessageID},
		time.Now(),
		issuerPublicKey,
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Buffered Queue test /////////////////////////////////////////////////////////////////////////////////////////////

const numMessages = 100

func TestSubmit(t *testing.T) {
	b := NewBufferQueue()
	var size int
	for i := 0; i < numMessages; i++ {
		msg := newMessage(identity.GenerateIdentity().PublicKey())
		size += len(msg.Bytes())
		assert.NoError(t, b.Submit(msg, 1))
		assert.EqualValues(t, i+1, b.NumActiveNodes())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestUnsubmit(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	assert.EqualValues(t, numMessages, b.NumActiveNodes())
	assert.EqualValues(t, numMessages, ringLen(b))
	for i := range messages {
		b.Unsubmit(messages[i])
		assert.EqualValues(t, numMessages-1-i, b.NumActiveNodes())
		assert.EqualValues(t, numMessages-1-i, ringLen(b))
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestReady(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	for i := range messages {
		assert.True(t, b.Ready(messages[i]))
		assert.False(t, b.Ready(messages[i]))
		assert.Equal(t, messages[i], b.PopFront())
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestTime(t *testing.T) {
	b := NewBufferQueue()

	future := newMessage(selfNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	assert.NoError(t, b.Submit(future, 1))
	assert.True(t, b.Ready(future))

	now := newMessage(selfNode.PublicKey())
	assert.NoError(t, b.Submit(now, 1))
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestRing(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
		b.Ready(messages[i])
	}
	for i := range messages {
		assert.Equal(t, messages[i], b.Current().Front())
		b.Next()
	}
	assert.Equal(t, messages[0], b.Current().Front())
}

func ringLen(b *BufferQueue) int {
	n := 0
	if q := b.Current(); q != nil {
		n = 1
		for b.Next() != q {
			n++
		}
	}
	return n
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
