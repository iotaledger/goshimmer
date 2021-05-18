package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestFifoScheduler(t *testing.T) {
	// create Scheduler dependencies
	// create the tangle
	tangle := newTestTangle()
	defer tangle.Shutdown()

	// setup tangle up till the FIFOScheduler
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.FIFOScheduler.Setup()
	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(tangle.FIFOScheduler.Schedule))
	tangle.FIFOScheduler.Start()

	// testing desired scheduled order: A - B - C - D - E  (B - A - D - C is equivalent)
	messages := make(map[string]*Message)
	messages["A"] = newTestDataMessage("A")
	messages["B"] = newTestDataMessage("B")
	messages["C"] = newTestParentsDataWithTimestamp("C", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now())
	messages["D"] = newTestParentsDataWithTimestamp("D", []MessageID{messages["A"].ID(), messages["B"].ID()}, []MessageID{}, time.Now().Add(100*time.Millisecond))
	messages["E"] = newTestParentsDataWithTimestamp("E", []MessageID{messages["C"].ID(), messages["D"].ID()}, []MessageID{}, time.Now().Add(3*time.Second))

	// Bypass the Booker
	tangle.FIFOScheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBooked(true)
			tangle.ConsensusManager.Events.MessageOpinionFormed.Trigger(messageID)
		})
	}))

	// store messages bypassing the messageStored event
	for _, message := range messages {
		tangle.Storage.StoreMessage(message)
	}

	assert.Eventually(t, func() (allMessagedScheduled bool) {
		allMessagedScheduled = true
		for _, message := range messages {
			tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
				allMessagedScheduled = messageMetadata.Scheduled()
			})

			if !allMessagedScheduled {
				return
			}
		}

		return allMessagedScheduled
	}, 10*time.Second, 100*time.Millisecond)
}
