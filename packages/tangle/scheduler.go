package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
)

const (
	outboxCapacity = 1024
	outboxWorkers  = 1
)

// region Scheduler ////////////////////////////////////////////////////////////////////////////////////////////////////

type Scheduler struct {
	Events *SchedulerEvents

	tangle         *Tangle
	outbox         chan MessageID
	outboxWP       async.WorkerPool
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

func NewScheduler(tangle *Tangle) (scheduler *Scheduler) {
	scheduler = &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(messageIDEventHandler),
		},

		tangle:         tangle,
		outbox:         make(chan MessageID, outboxCapacity),
		shutdownSignal: make(chan struct{}),
	}

	scheduler.outboxWP.Tune(outboxWorkers)
	scheduler.run()

	return
}

func (s *Scheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(s.Schedule))
}

func (s *Scheduler) Schedule(messageID MessageID) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if !s.parentsBooked(message) {
			return
		}

		s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			if messageMetadata.SetScheduled(true) {
				s.outbox <- messageID
			}
		})
	})
}

func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})
}

func (s *Scheduler) run() {
	go func() {
		for {
			select {
			case <-s.shutdownSignal:
				return
			case scheduledMessageID := <-s.outbox:
				s.outboxWP.Submit(func() {
					s.Events.MessageScheduled.Trigger(scheduledMessageID)

					s.tangle.Utils.WalkMessageAndMetadata(func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker) {
						if !s.parentsBooked(message) {
							return
						}

						if messageMetadata.SetScheduled(true) {
							s.Events.MessageScheduled.Trigger(message.ID())

							for _, childMessageID := range s.tangle.Utils.ApprovingMessageIDs(message.ID()) {
								walker.Push(childMessageID)
							}
						}
					}, s.tangle.Utils.ApprovingMessageIDs(scheduledMessageID), true)
				})
			}
		}
	}()
}

func (s *Scheduler) parentsBooked(message *Message) (parentsBooked bool) {
	parentsBooked = true
	message.ForEachParent(func(parent Parent) {
		if !parentsBooked {
			return
		}

		if parent.ID == EmptyMessageID {
			return
		}

		s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
			parentsBooked = messageMetadata.IsBooked()
		})
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
