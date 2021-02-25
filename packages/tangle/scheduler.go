package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

const (
	inboxCapacity = 1024
)

// region Scheduler ////////////////////////////////////////////////////////////////////////////////////////////////////

type Scheduler struct {
	Events *SchedulerEvents

	tangle            *Tangle
	inbox             chan MessageID
	outboxWorkersDone sync.WaitGroup
	shutdownSignal    chan struct{}
	runDone           sync.WaitGroup
	shutdownOnce      sync.Once
}

func NewScheduler(tangle *Tangle) (scheduler *Scheduler) {
	scheduler = &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(messageIDEventHandler),
		},

		tangle:         tangle,
		inbox:          make(chan MessageID, inboxCapacity),
		shutdownSignal: make(chan struct{}),
	}
	scheduler.run()

	return
}

func (s *Scheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(s.Schedule))
}

func (s *Scheduler) Schedule(messageID MessageID) {
	s.inbox <- messageID
}

func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})

	s.runDone.Wait()
	s.outboxWorkersDone.Wait()
}

func (s *Scheduler) run() {
	s.runDone.Add(1)
	go func() {
		defer s.runDone.Done()

		for {
			select {
			case <-s.shutdownSignal:
				return
			case messageID := <-s.inbox:
				if !s.parentsBooked(messageID) {
					continue
				}

				s.Events.MessageScheduled.Trigger(messageID)
			}
		}
	}()
}

func (s *Scheduler) parentsBooked(messageID MessageID) (parentsBooked bool) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		parentsBooked = true
		message.ForEachParent(func(parent Parent) {
			if !parentsBooked || parent.ID == EmptyMessageID {
				return
			}

			if !s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
				parentsBooked = messageMetadata.IsBooked()
			}) {
				parentsBooked = false
			}
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
