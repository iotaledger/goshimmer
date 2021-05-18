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

// FIFOScheduler is a Tangle component that takes care of scheduling the messages that shall be booked.
type FIFOScheduler struct {
	Events *FIFOSchedulerEvents

	tangle                 *Tangle
	inbox                  chan MessageID
	scheduledMessages      set.Set
	allMessagesScheduledWG sync.WaitGroup
	shutdownSignal         chan struct{}
	shutdown               sync.WaitGroup
	shutdownOnce           sync.Once
	onOpinionFormed        *events.Closure
	onMessageInvalid       *events.Closure
}

// NewFIFOScheduler returns a new scheduler.
func NewFIFOScheduler(tangle *Tangle) *FIFOScheduler {
	scheduler := &FIFOScheduler{
		Events: &FIFOSchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
		},

		tangle:            tangle,
		inbox:             make(chan MessageID, inboxCapacity),
		shutdownSignal:    make(chan struct{}),
		scheduledMessages: set.New(true),
	}
	scheduler.onMessageInvalid = events.NewClosure(scheduler.messageInvalidHandler)
	scheduler.onOpinionFormed = events.NewClosure(scheduler.opinionFormedHandler)

	return scheduler
}

// Start starts the scheduler.
func (s *FIFOScheduler) Start() {
	// start the main loop
	go s.mainLoop()
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *FIFOScheduler) Setup() {
	s.tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(s.onOpinionFormed)
	s.tangle.Events.MessageInvalid.Attach(s.onMessageInvalid)
}

// Schedule schedules the given messageID.
func (s *FIFOScheduler) Schedule(messageID MessageID) {
	s.inbox <- messageID
}

// Shutdown shuts down the scheduler and persists its state.
func (s *FIFOScheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})

	s.shutdown.Wait()
	s.allMessagesScheduledWG.Wait()
	s.tangle.ConsensusManager.Events.MessageOpinionFormed.Detach(s.onOpinionFormed)
	s.tangle.Events.MessageInvalid.Detach(s.onMessageInvalid)
}

func (s *FIFOScheduler) mainLoop() {
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
}

func (s *FIFOScheduler) scheduleMessage(messageID MessageID) {
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

// FIFOSchedulerEvents represents events happening in the Scheduler.
type FIFOSchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
	MessageDiscarded *events.Event
	NodeBlacklisted  *events.Event
}

func (s *FIFOScheduler) messageInvalidHandler(messageID MessageID) {
	if s.scheduledMessages.Delete(messageID) {
		s.allMessagesScheduledWG.Done()
	}
}

func (s *FIFOScheduler) opinionFormedHandler(messageID MessageID) {
	if s.scheduledMessages.Delete(messageID) {
		s.allMessagesScheduledWG.Done()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
