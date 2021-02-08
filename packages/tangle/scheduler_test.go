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
	tangle := &Tangle{
		Events: newEvents(),
	}
	tangle.Storage = newMessageStore()
	tangle.Solidifier = NewSolidifier(tangle)

	// create and start the Scheduler
	testScheduler := NewScheduler(tangle)

	// testing desired scheduled order: A - B - D - C  (B - A - D - C is equivalent)
	messages := make(map[string]*Message)
	messages["A"] = newTestDataMessage("A")
	messages["B"] = newTestDataMessage("B")
	// set C to have a timestamp in the future
	messages["C"] = newTestParentsDataWithTimestamp("C", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now().Add(15*time.Second))
	messages["D"] = newTestParentsDataWithTimestamp("D", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now())

	// The order of A and B cannot be guaranteed and it does not matter.
	expectedOrder := []MessageID{messages["D"].ID(), messages["C"].ID()}
	scheduledOrder := []MessageID{}
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
		wg.Done()
		scheduledOrderMutex.Lock()
		scheduledOrder = append(scheduledOrder, messageID)
		scheduledOrderMutex.Unlock()
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.Events.MessageBooked.Trigger(messageID)
		})
		tangle.Storage.Message(messageID).Consume(func(message *Message) {
			assert.True(t, !clock.SyncedTime().Before(message.IssuingTime()))
		})
	}))

	wg.Add(4)
	// Trigger solid events not in order
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["A"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["C"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["D"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["B"].ID())

	wg.Wait()
	testScheduler.Stop()

	// messageID-string mapping for debugging.
	IDMap := map[MessageID]string{
		messages["A"].ID(): "A",
		messages["B"].ID(): "B",
		messages["C"].ID(): "C",
		messages["D"].ID(): "D",
	}

	// assert that messages A and B are scheduled before D, and D before C
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
		&messageToSchedule{issuingTime: now},
		&messageToSchedule{issuingTime: now.Add(1 * time.Second)},
		&messageToSchedule{issuingTime: now.Add(3 * time.Second)},
	}

	before := &messageToSchedule{issuingTime: now.Add(-1 * time.Second)}
	list.insert(before)
	assert.Equal(t, before, list[0])

	after := &messageToSchedule{issuingTime: now.Add(5 * time.Second)}
	list.insert(after)
	assert.Equal(t, after, list[len(list)-1])

	between := &messageToSchedule{issuingTime: now.Add(2 * time.Second)}
	list.insert(between)
	assert.Equal(t, between, list[3])

}
