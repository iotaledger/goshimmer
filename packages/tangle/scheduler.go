package tangle

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
)

const (
	// MaxDeficit is the maximum deficit, i.e. max bytes that can be scheduled without waiting; >= maxMessageSize
	MaxDeficit = MaxMessageSize
)

// rate is the minimum time interval between two scheduled messages, i.e. 1s / MPS
var rate = time.Second / 200

// AccessManaRetrieveFunc is a function type to retrieve access mana (e.g. via the mana plugin)
type AccessManaRetrieveFunc func(nodeID identity.ID) float64

// TotalAccessManaRetrieveFunc is a function type to retrieve the total access mana (e.g. via the mana plugin)
type TotalAccessManaRetrieveFunc func() float64

// SchedulerParams defines the scheduler config parameters.
type SchedulerParams struct {
	Rate                        time.Duration
	MaxQueueWeight              *float64
	AccessManaRetrieveFunc      func(identity.ID) float64
	TotalAccessManaRetrieveFunc func() float64
}

// Scheduler is a Tangle component that takes care of scheduling the messages that shall be booked.
type Scheduler struct {
	Events           *SchedulerEvents
	tangle           *Tangle
	self             identity.ID
	mu               sync.Mutex
	buffer           *schedulerutils.BufferQueue
	deficits         map[identity.ID]float64
	onMessageSolid   *events.Closure
	onMessageInvalid *events.Closure
	shutdownSignal   chan struct{}
	shutdownOnce     sync.Once
	ticker           *time.Ticker
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) *Scheduler {
	if tangle.Options.SchedulerParams.AccessManaRetrieveFunc == nil || tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc == nil {
		panic("the option AccessManaRetriever and TotalAccessManaRetriever must be defined so that AccessMana can be determined in scheduler")
	}
	if tangle.Options.SchedulerParams.MaxQueueWeight != nil {
		schedulerutils.MaxQueueWeight = *tangle.Options.SchedulerParams.MaxQueueWeight
	}
	if tangle.Options.SchedulerParams.Rate > 0 {
		rate = tangle.Options.SchedulerParams.Rate
	}

	scheduler := &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
		},
		self:           tangle.Options.Identity.ID(),
		tangle:         tangle,
		buffer:         schedulerutils.NewBufferQueue(),
		deficits:       make(map[identity.ID]float64),
		shutdownSignal: make(chan struct{}),
	}
	scheduler.onMessageSolid = events.NewClosure(scheduler.onMessageSolidHandler)
	scheduler.onMessageInvalid = events.NewClosure(scheduler.onMessageInvalidHandler)
	return scheduler
}

func (s *Scheduler) onMessageSolidHandler(messageID MessageID) {
	s.SubmitAndReadyMessage(messageID)
}

func (s *Scheduler) onMessageInvalidHandler(messageID MessageID) {
	s.Unsubmit(messageID)
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	go s.mainLoop()
}

// Detach detaches the scheduler from the tangle events.
func (s *Scheduler) Detach() {
	s.tangle.Solidifier.Events.MessageSolid.Detach(s.onMessageSolid)
	s.tangle.Events.MessageInvalid.Detach(s.onMessageInvalid)
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(s.onMessageSolid)
	s.tangle.Events.MessageInvalid.Attach(s.onMessageInvalid)

	//  TODO: wait for all messages to be scheduled here or in message layer?
	/*
		s.tangle.ConsensusManager.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
			if s.scheduledMessages.Delete(messageID) {
				s.allMessagesScheduledWG.Done()
			}
		}))
	*/
}

// SubmitAndReadyMessage submits the message to the scheduler and makes it ready when it's parents are booked.
func (s *Scheduler) SubmitAndReadyMessage(messageID MessageID) {
	err := s.Submit(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler Submit: %v", err))
		return
	}

	err = s.Ready(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler Ready: %v", err))
		return
	}
}

// SetRate sets the rate of the scheduler.
func (s *Scheduler) SetRate(rate time.Duration) {
	s.ticker = time.NewTicker(rate)
	s.tangle.Options.SchedulerParams.Rate = rate
}

// Shutdown shuts down the Scheduler.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownSignal)
	})
}

// Submit submits a message to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(messageID) has been called.
func (s *Scheduler) Submit(messageID MessageID) error {
	var err error

	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()

		nodeID := identity.NewID(message.IssuerPublicKey())
		// get the current access mana inside the lock
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(nodeID)
		if mana <= 0 {
			err = schedulerutils.ErrInvalidMana
			s.Events.MessageDiscarded.Trigger(messageID)
			return
		}

		err = s.buffer.Submit(message, mana)
		if err != nil {
			s.Events.MessageDiscarded.Trigger(messageID)
		}
		if errors.Is(err, schedulerutils.ErrInboxExceeded) {
			s.Events.NodeBlacklisted.Trigger(nodeID)
		}
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}

	return err
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (s *Scheduler) Unsubmit(messageID MessageID) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Unsubmit(message)
	}) {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler (Unsubmit): failed to get message '%x' from storage", messageID))
	}
}

// Ready marks a previously submitted message as ready to be scheduled.
// If Ready is called without a previous Submit, it has no effect.
func (s *Scheduler) Ready(messageID MessageID) error {
	var err error

	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Ready(message)
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}

	return err
}

// RemoveNode removes all messages (submitted and ready) for the given node.
func (s *Scheduler) RemoveNode(nodeID identity.ID) (err error) {
	if nodeID == s.self {
		return xerrors.Errorf("Invalid node to remove")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: is it necessary to trigger MessageDiscarded for all the removed messages.
	s.buffer.RemoveNode(nodeID)
	return
}

func (s *Scheduler) parentsBooked(messageID MessageID) (parentsBooked bool) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		parentsBooked = true
		message.ForEachParent(func(parent Parent) {
			if !parentsBooked || parent.ID == EmptyMessageID {
				return
			}

			if !s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
				parentsBooked = parentsBooked && messageMetadata.IsBooked()
			}) {
				parentsBooked = false
			}
		})
	})

	return
}

func (s *Scheduler) schedule() *Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	// no messages submitted
	if s.buffer.Size() == 0 {
		return nil
	}

	start := s.buffer.Current()
	now := time.Now()
	for q := start; ; {
		f := q.Front() // check whether the front of the queue can be scheduled
		if f != nil && s.getDeficit(q.NodeID()) >= float64(len(f.Bytes())) && !now.Before(f.IssuingTime()) {
			break
		}
		// otherwise increase the deficit
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(q.NodeID())
		err := s.setDeficit(q.NodeID(), s.getDeficit(q.NodeID())+mana)
		if err != nil {
			s.tangle.Events.Error.Trigger(err)
			return nil
		}

		// TODO: different from spec
		q = s.buffer.Next()
		if q == nil {
			s.tangle.Events.Error.Trigger(xerrors.Errorf("error in Scheduler (schedule): failed to get next message from bufferQueue"))
			return nil
		}
		if q == start {
			return nil
		}
	}

	// will stay in
	if !s.parentsBooked(s.buffer.Current().Front().(*Message).ID()) {
		return nil
	}
	msg := s.buffer.PopFront()
	nodeID := identity.NewID(msg.IssuerPublicKey())
	err := s.setDeficit(nodeID, s.getDeficit(nodeID)-float64(len(msg.Bytes())))
	if err != nil {
		s.tangle.Events.Error.Trigger(err)
		return nil
	}

	return msg.(*Message)
}

// mainLoop periodically triggers the scheduling of ready messages.
func (s *Scheduler) mainLoop() {
	s.ticker = time.NewTicker(rate)
	defer s.ticker.Stop()

	for {
		select {
		// every rate time units
		case <-s.ticker.C:
			// TODO: do we need to pause the ticker, if there are no ready messages
			msg := s.schedule()
			if msg != nil {
				s.Events.MessageScheduled.Trigger(msg.ID())
			}

		// on close, exit the loop
		case <-s.shutdownSignal:
			return
		}
	}
}

func (s *Scheduler) getDeficit(nodeID identity.ID) float64 {
	return s.deficits[nodeID]
}

func (s *Scheduler) setDeficit(nodeID identity.ID, deficit float64) (err error) {
	if deficit < 0 {
		return xerrors.Errorf("error in Scheduler (setDeficit): deficit is less than 0")
	}
	s.deficits[nodeID] = math.Min(deficit, MaxDeficit)

	return
}

// NodeQueueSize returns the size of the nodeIDs queue.
func (s *Scheduler) NodeQueueSize(nodeID identity.ID) uint {
	return s.buffer.NodeQueue(nodeID).Size()
}

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
	MessageDiscarded *events.Event
	NodeBlacklisted  *events.Event
}

// NodeIDCaller is the caller function for events that hand over a NodeID.
func NodeIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(identity.ID))(params[0].(identity.ID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
