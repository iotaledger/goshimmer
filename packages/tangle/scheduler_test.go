package tangle

import (
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
)

// region Scheduler_test /////////////////////////////////////////////////////////////////////////////////////////////

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
	peerNode          = identity.GenerateIdentity()
)

func TestScheduler_StartStop(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	tangle.Scheduler.Start()

	time.Sleep(10 * time.Millisecond)
	tangle.Scheduler.Shutdown()
}

func TestScheduler_Submit(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()
	tangle.Scheduler.Start()

	msg := newMessage(selfNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	assert.NoError(t, tangle.Scheduler.Submit(msg.ID()))
	time.Sleep(100 * time.Millisecond)
	// unsubmit to allow the scheduler to shutdown
	assert.NoError(t, tangle.Scheduler.Unsubmit(msg.ID()))
}

func TestScheduler_Discarded(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	messageDiscarded := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageDiscarded.Attach(events.NewClosure(func(id MessageID) { messageDiscarded <- id }))

	tangle.Scheduler.Start()

	// this node has no mana so the message will be discarded
	msg := newMessage(noAManaNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	err := tangle.Scheduler.Submit(msg.ID())
	assert.Truef(t, errors.Is(err, schedulerutils.ErrInsufficientMana), "unexpected error: %v", err)

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageDiscarded:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_DiscardedAtShutdown(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	messageDiscarded := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageDiscarded.Attach(events.NewClosure(func(id MessageID) { messageDiscarded <- id }))

	tangle.Scheduler.Start()

	msg := newMessage(selfNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	assert.NoError(t, tangle.Scheduler.Submit(msg.ID()))

	time.Sleep(100 * time.Millisecond)
	tangle.Scheduler.Shutdown()

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageDiscarded:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_SetRateBeforeStart(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Scheduler.SetRate(time.Hour)
	tangle.Scheduler.Start()
	tangle.Scheduler.SetRate(testRate)
}

func TestScheduler_Schedule(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	messageScheduled := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	tangle.Scheduler.Start()

	// create a new message from a different node
	msg := newMessage(peerNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	assert.NoError(t, tangle.Scheduler.Submit(msg.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(msg.ID()))

	assert.Eventually(t, func() bool {
		select {
		case id := <-messageScheduled:
			return assert.Equal(t, msg.ID(), id)
		default:
			return false
		}
	}, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_SetRate(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	var scheduled atomic.Bool
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(MessageID) { scheduled.Store(true) }))

	tangle.Scheduler.Start()

	// effectively disable the scheduler by setting a very low rate
	tangle.Scheduler.SetRate(time.Hour)
	// assure that any potential ticks issued before the rate change have been processed
	time.Sleep(100 * time.Millisecond)

	// submit a new message to the scheduler
	msg := newMessage(peerNode.PublicKey())
	tangle.Storage.StoreMessage(msg)
	assert.NoError(t, tangle.Scheduler.Submit(msg.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(msg.ID()))

	// the message should not be scheduled as the rate is too low
	time.Sleep(100 * time.Millisecond)
	assert.False(t, scheduled.Load())

	// after reducing the rate again, the message should eventually be scheduled
	tangle.Scheduler.SetRate(10 * time.Millisecond)
	assert.Eventually(t, scheduled.Load, 1*time.Second, 10*time.Millisecond)
}

func TestScheduler_Time(t *testing.T) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	messageScheduled := make(chan MessageID, 1)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	tangle.Scheduler.Start()

	future := newMessage(peerNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	tangle.Storage.StoreMessage(future)
	assert.NoError(t, tangle.Scheduler.Submit(future.ID()))

	now := newMessage(peerNode.PublicKey())
	tangle.Storage.StoreMessage(now)
	assert.NoError(t, tangle.Scheduler.Submit(now.ID()))

	assert.NoError(t, tangle.Scheduler.Ready(future.ID()))
	assert.NoError(t, tangle.Scheduler.Ready(now.ID()))

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
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(id MessageID) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(id))
	}))
	tangle.Scheduler.Start()

	const numMessages = 5
	messageScheduled := make(chan MessageID, numMessages)
	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(id MessageID) { messageScheduled <- id }))

	ids := make([]MessageID, numMessages)
	for i := range ids {
		msg := newMessage(selfNode.PublicKey())
		tangle.Storage.StoreMessage(msg)
		ids[i] = msg.ID()
	}

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
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(id MessageID) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(id))
	}))
	tangle.Scheduler.Start()

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

func TestSchedulerParallelSubmit(t *testing.T) {
	const (
		totalMsgCount = 200
		tangleWidth   = 250
		networkDelay  = 5 * time.Millisecond
	)

	var totalScheduled atomic.Int32

	// create Scheduler dependencies
	// create the tangle
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	tangle.Events.Error.Attach(events.NewClosure(func(err error) { assert.Failf(t, "unexpected error", "error event triggered: %v", err) }))

	// setup tangle up till the Scheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Scheduler.Setup()
	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(id MessageID) {
		assert.NoError(t, tangle.Scheduler.SubmitAndReady(id))
	}))
	tangle.Scheduler.Start()

	// generate the messages we want to solidify
	messages := make(map[MessageID]*Message, totalMsgCount)
	for i := 0; i < totalMsgCount/2; i++ {
		msg := newMessage(selfNode.PublicKey())
		messages[msg.ID()] = msg
	}

	for i := 0; i < totalMsgCount/2; i++ {
		msg := newMessage(peerNode.PublicKey())
		messages[msg.ID()] = msg
	}

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		t.Logf(messageID.Base58(), " solid")
	}))

	tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		n := totalScheduled.Add(1)
		t.Logf("scheduled messages %d/%d", n, totalMsgCount)
	}))

	// issue tips to start solidification
	t.Run("ParallelSubmit", func(t *testing.T) {
		for _, m := range messages {
			t.Run(m.ID().Base58(), func(t *testing.T) {
				m := m
				t.Parallel()
				t.Logf("issue message: %s", m.ID().Base58())
				tangle.Storage.StoreMessage(m)
			})
		}
	})

	// wait for all messages to have a formed opinion
	assert.Eventually(t, func() bool { return totalScheduled.Load() == totalMsgCount }, 5*time.Minute, 100*time.Millisecond)
}

func BenchmarkScheduler(b *testing.B) {
	tangle := newTestTangle(Identity(selfLocalIdentity))
	defer tangle.Shutdown()

	msg := newMessage(selfNode.PublicKey())
	tangle.Storage.StoreMessage(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := tangle.Scheduler.SubmitAndReady(msg.ID()); err != nil {
			b.Fatal(err)
		}
		tangle.Scheduler.schedule()
	}
	b.StopTimer()
}

var timeOffset = time.Nanosecond

func newMessage(issuerPublicKey ed25519.PublicKey) *Message {
	timeOffset += time.Nanosecond

	return NewMessage(
		[]MessageID{EmptyMessageID},
		[]MessageID{},
		time.Now().Add(timeOffset),
		issuerPublicKey,
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
