package tangle

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestScheduler(t *testing.T) {
	// create Scheduler dependencies
	// create the tangle
	tangle := New()
	defer tangle.Shutdown()

	// start the Scheduler
	testScheduler := tangle.Scheduler
	testScheduler.Setup()

	// testing desired scheduled order: A - B - D - C  (B - A - D - C is equivalent)
	messages := make(map[string]*Message)
	messages["A"] = newTestDataMessage("A")
	messages["B"] = newTestDataMessage("B")
	// set C to have a timestamp in the future
	messages["C"] = newTestParentsDataWithTimestamp("C", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now().Add(5*time.Second))
	messages["D"] = newTestParentsDataWithTimestamp("D", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now())
	messages["E"] = newTestParentsDataWithTimestamp("E", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now().Add(3*time.Second))

	scheduledOrder := make([]MessageID, 0)
	var scheduledOrderMutex sync.Mutex

	// store messages bypassing the messageStored event
	for _, message := range messages {
		storedMetadata, stored := tangle.Storage.messageMetadataStorage.StoreIfAbsent(NewMessageMetadata(message.ID()))
		if !stored {
			return
		}
		storedMetadata.Release()
		tangle.Storage.messageStorage.Store(message).Release()
	}

	var wg sync.WaitGroup

	// Bypass the Booker
	testScheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		scheduledOrderMutex.Lock()
		scheduledOrder = append(scheduledOrder, messageID)
		scheduledOrderMutex.Unlock()
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.Booker.Events.MessageBooked.Trigger(messageID)
		})
		tangle.Storage.Message(messageID).Consume(func(message *Message) {
			assert.True(t, !clock.SyncedTime().Before(message.IssuingTime()))
		})
		wg.Done()
	}))

	wg.Add(5)
	// Trigger solid events not in order
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["A"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["C"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["D"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["B"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["E"].ID())

	wg.Wait()

	// messageID-string mapping for debugging.
	IDMap := map[MessageID]string{
		messages["A"].ID(): "A",
		messages["B"].ID(): "B",
		messages["C"].ID(): "C",
		messages["D"].ID(): "D",
		messages["E"].ID(): "E",
	}

	// The order of A and B cannot be guaranteed and it does not matter.
	expectedOrder := []MessageID{messages["D"].ID(), messages["E"].ID(), messages["C"].ID()}

	// assert that messages A and B are scheduled before D, and D before E, and E before C
	equal := assert.Equal(t, expectedOrder[:], scheduledOrder[2:])
	if !equal {
		for _, msgID := range scheduledOrder {
			t.Log(IDMap[msgID])
		}
	}
}

func TestTimeIssuanceSortedList(t *testing.T) {
	now := time.Now()
	list := timeIssuanceSortedList{
		&MessageToSchedule{issuingTime: now},
		&MessageToSchedule{issuingTime: now.Add(1 * time.Second)},
		&MessageToSchedule{issuingTime: now.Add(3 * time.Second)},
	}

	before := &MessageToSchedule{issuingTime: now.Add(-1 * time.Second)}
	list.insert(before)
	assert.Equal(t, before, list[0])

	after := &MessageToSchedule{issuingTime: now.Add(5 * time.Second)}
	list.insert(after)
	assert.Equal(t, after, list[len(list)-1])

	between := &MessageToSchedule{issuingTime: now.Add(2 * time.Second)}
	list.insert(between)
	assert.Equal(t, between, list[3])

}
