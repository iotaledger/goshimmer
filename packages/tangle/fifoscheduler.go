package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
)

const (
	inboxCapacity = 64
)

// region Scheduler ////////////////////////////////////////////////////////////////////////////////////////////////////

// FifoScheduler is a Tangle component that takes care of scheduling the messages that shall be booked.
type FifoScheduler struct {
	Events *FifoSchedulerEvents

	tangle                 *Tangle
	inbox                  chan MessageID
	scheduledMessages      set.Set
	allMessagesScheduledWG sync.WaitGroup
	shutdownSignal         chan struct{}
	shutdown               sync.WaitGroup
	shutdownOnce           sync.Once
	onMessageSolid         *events.Closure
	onOpinionFormed        *events.Closure
	onMessageInvalid       *events.Closure
}

// NewFifoScheduler returns a new scheduler.
func NewFifoScheduler(tangle *Tangle) (scheduler *FifoScheduler) {
	scheduler = &FifoScheduler{
		Events: &FifoSchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
		},

		tangle:            tangle,
		inbox:             make(chan MessageID, inboxCapacity),
		shutdownSignal:    make(chan struct{}),
		scheduledMessages: set.New(true),
	}
	scheduler.onMessageSolid = events.NewClosure(scheduler.messageSolidHandler)
	scheduler.onMessageInvalid = events.NewClosure(scheduler.messageInvalidHandler)
	scheduler.onOpinionFormed = events.NewClosure(scheduler.opinionFormedHandler)
	scheduler.run()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *FifoScheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(s.onMessageSolid)
	s.tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(s.onOpinionFormed)
	s.tangle.Events.MessageInvalid.Attach(s.onMessageInvalid)
}

// Detach detaches the scheduler from the tangle events.
func (s *FifoScheduler) Detach() {
	s.tangle.Solidifier.Events.MessageSolid.Detach(s.onMessageSolid)
}

// Schedule schedules the given messageID.
func (s *FifoScheduler) Schedule(messageID MessageID) {
	s.inbox <- messageID
}

// Shutdown shuts down the Scheduler and persists its state.
func (s *FifoScheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})

	s.shutdown.Wait()
	s.allMessagesScheduledWG.Wait()
	s.tangle.ConsensusManager.Events.MessageOpinionFormed.Detach(s.onOpinionFormed)
	s.tangle.Events.MessageInvalid.Detach(s.onMessageInvalid)
}

func (s *FifoScheduler) run() {
	go func() {
		for {
			select {
			case messageID := <-s.inbox:
				s.scheduleMessage(messageID)
			case <-s.shutdownSignal:
				if len(s.inbox) == 0 {
					return
				}
			}
		}
	}()
}

func (s *FifoScheduler) scheduleMessage(messageID MessageID) {
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if messageMetadata.SetScheduled(true) {
			if s.scheduledMessages.Add(messageID) {
				s.allMessagesScheduledWG.Add(1)
			}
			s.Events.MessageScheduled.Trigger(messageID)
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// FifoSchedulerEvents represents events happening in the Scheduler.
type FifoSchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
	MessageDiscarded *events.Event
	NodeBlacklisted  *events.Event
}

func (s *FifoScheduler) messageSolidHandler(messageID MessageID) {
	s.Schedule(messageID)
}

func (s *FifoScheduler) messageInvalidHandler(messageID MessageID) {
	if s.scheduledMessages.Delete(messageID) {
		s.allMessagesScheduledWG.Done()
	}
}

func (s *FifoScheduler) opinionFormedHandler(messageID MessageID) {
	if s.scheduledMessages.Delete(messageID) {
		s.allMessagesScheduledWG.Done()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
