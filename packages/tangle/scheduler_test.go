package tangle

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestScheduler(t *testing.T) {
	tangle := &Tangle{
		Events: newEvents(),
	}
	tangle.Storage = newMessageStore()
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Events.MessageBooked = events.NewEvent(messageIDEventHandler)

	testScheduler := NewScheduler(tangle)

	testScheduler.Start()

	var wg sync.WaitGroup

	messages := map[string]*Message{
		"A": newTestDataMessage("A"),
		"B": newTestDataMessage("B"),
	}
	messages["C"] = newTestParentsDataWithTimestamp("C", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now().Add(1*time.Second))
	messages["D"] = newTestParentsDataWithTimestamp("D", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now())

	for _, message := range messages {
		tangle.Storage.StoreMessage(message)
	}

	testScheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		wg.Done()
		t.Log("Message Booked: ", messageID)
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.Events.MessageBooked.Trigger(messageID)
		})
	}))

	wg.Add(4)
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["A"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["C"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["D"].ID())
	tangle.Solidifier.Events.MessageSolid.Trigger(messages["B"].ID())

	wg.Wait()
	testScheduler.Stop()

}

func TestTimeIssuanceSortedList(t *testing.T) {
	now := time.Now()
	list := timeIssuanceSortedList{
		&Message{issuingTime: now},
		&Message{issuingTime: now.Add(1 * time.Second)},
		&Message{issuingTime: now.Add(3 * time.Second)},
	}

	before := &Message{issuingTime: now.Add(-1 * time.Second)}
	list = list.insert(before)
	assert.Equal(t, before, list[0])

	after := &Message{issuingTime: now.Add(5 * time.Second)}
	list = list.insert(after)
	assert.Equal(t, after, list[len(list)-1])

	between := &Message{issuingTime: now.Add(2 * time.Second)}
	list = list.insert(between)
	assert.Equal(t, between, list[3])

}
