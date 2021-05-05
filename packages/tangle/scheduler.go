package tangle

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/workerpool"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

const (
	// MaxDeficit is the maximum deficit, i.e. max bytes that can be scheduled without waiting; >= maxMessageSize
	MaxDeficit = MaxMessageSize
)

// ErrNotRunning is returned when a message is submitted when the scheduler has not been started
var ErrNotRunning = xerrors.New("scheduler is not running")

var (
	submitWorkerCount     = 4
	submitWorkerQueueSize = 250
	submitWorkerPool      *workerpool.WorkerPool
)

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
	running          atomic.Bool
	wg               sync.WaitGroup
	ticker           *time.Ticker
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) *Scheduler {
	if tangle.Options.SchedulerParams.AccessManaRetrieveFunc == nil || tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc == nil {
		panic("scheduler: the option AccessManaRetriever and TotalAccessManaRetriever must be defined so that AccessMana can be determined in scheduler")
	}
	if tangle.Options.SchedulerParams.MaxQueueWeight != nil {
		schedulerutils.MaxQueueWeight = *tangle.Options.SchedulerParams.MaxQueueWeight
	}

	scheduler := &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
		},
		self:     tangle.Options.Identity.ID(),
		tangle:   tangle,
		buffer:   schedulerutils.NewBufferQueue(),
		deficits: make(map[identity.ID]float64),
	}
	scheduler.onMessageSolid = events.NewClosure(scheduler.onMessageSolidHandler)
	scheduler.onMessageInvalid = events.NewClosure(scheduler.onMessageInvalidHandler)

	submitWorkerPool = workerpool.New(func(task workerpool.Task) {
		scheduler.SubmitAndReadyMessage(task.Param(0).(MessageID))
		task.Return(nil)
	}, workerpool.WorkerCount(submitWorkerCount), workerpool.QueueSize(submitWorkerQueueSize))

	return scheduler
}

func (s *Scheduler) onMessageSolidHandler(messageID MessageID) {
	submitWorkerPool.TrySubmit(messageID)
}

func (s *Scheduler) onMessageInvalidHandler(messageID MessageID) {
	// unsubmit the message
	err := s.Unsubmit(messageID)
	s.tangle.Events.Error.Trigger(xerrors.Errorf("failed to unsubmit: %w", err))
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	// create the ticker here to assure that SetRate never hits an uninitialized ticker
	s.ticker = time.NewTicker(s.tangle.Options.SchedulerParams.Rate)
	// start the main loop
	s.wg.Add(1)
	go s.mainLoop()

	// start schedule queued messages
	fmt.Println("start workerpool queued size: ", submitWorkerPool.GetPendingQueueSize())
	submitWorkerPool.Start()

	s.running.Store(true)
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	s.running.Store(false)
	s.wg.Wait()
	submitWorkerPool.Stop()
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
}

// SubmitAndReadyMessage submits the message to the scheduler and makes it ready when it's parents are booked.
func (s *Scheduler) SubmitAndReadyMessage(messageID MessageID) {
	// submit the message to the scheduler and marks it ready right away
	err := s.Submit(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("failed to submit: %w", err))
		return
	}
	err = s.Ready(messageID)
	if err != nil {
		s.tangle.Events.Error.Trigger(xerrors.Errorf("failed to ready: %w", err))
		return
	}
}

// SetRate sets the rate of the scheduler.
func (s *Scheduler) SetRate(rate time.Duration) {
	s.tangle.Options.SchedulerParams.Rate = rate
	// only update the ticker when the scheduler is running
	if s.running.Load() {
		s.ticker.Reset(rate)
	}
}

// Submit submits a message to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(messageID) has been called.
func (s *Scheduler) Submit(messageID MessageID) (err error) {
	if !s.running.Load() {
		return ErrNotRunning
	}

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
		if xerrors.Is(err, schedulerutils.ErrInboxExceeded) {
			s.Events.NodeBlacklisted.Trigger(nodeID)
		}
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
}

// Unsubmit removes a message from the submitted messages.
// If that message is already marked as ready, Unsubmit has no effect.
func (s *Scheduler) Unsubmit(messageID MessageID) (err error) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Unsubmit(message)
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
}

// Ready marks a previously submitted message as ready to be scheduled.
// If Ready is called without a previous Submit, it has no effect.
func (s *Scheduler) Ready(messageID MessageID) (err error) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.buffer.Ready(message)
	}) {
		err = xerrors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
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

	eligibleNode := false
	start := s.buffer.Current()
	now := time.Now()
	for q := start; ; {
		msg := q.Front()
		// a message can be scheduled, if it is ready and its issuing time is not in the future
		valid := msg != nil && !now.Before(msg.IssuingTime())
		// if the current node has enough deficit and the first message in its outbox is valid, we are done
		if valid && s.getDeficit(q.NodeID()) >= float64(len(msg.Bytes())) {
			break
		}
		// otherwise increase its deficit
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(q.NodeID())
		// TODO: introduce positive amount of mana as minimum threshold
		if mana > 0 {
			s.updateDeficit(q.NodeID(), mana)
			if valid {
				eligibleNode = true
			}
		}

		// progress tho the next node that has ready messages
		q = s.buffer.Next()
		// if we reached the first node again without seeing any eligible nodes, break to prevent infinite loops
		if !eligibleNode && q == start {
			// TODO: this eligibility check is not in the spec and should be added
			return nil
		}
	}

	// if the parents are not booked yet, do not schedule the message
	// TODO: eventually this should be handled outside the scheduler by only calling Ready() when the parents are booked
	if !s.parentsBooked(s.buffer.Current().Front().(*Message).ID()) {
		return nil
	}
	// remove the message from the buffer and adjust node's deficit
	msg := s.buffer.PopFront()
	nodeID := identity.NewID(msg.IssuerPublicKey())
	s.updateDeficit(nodeID, -float64(len(msg.Bytes())))

	return msg.(*Message)
}

// mainLoop periodically triggers the scheduling of ready messages.
func (s *Scheduler) mainLoop() {
	defer s.wg.Done()
	defer s.ticker.Stop()

	for {
		select {
		// every rate time units
		case <-s.ticker.C:
			// TODO: pause the ticker, if there are no ready messages
			if msg := s.schedule(); msg != nil {
				s.Events.MessageScheduled.Trigger(msg.ID())
			}
			if !s.running.Load() && s.buffer.Size() == 0 {
				return
			}
		}
	}
}

func (s *Scheduler) getDeficit(nodeID identity.ID) float64 {
	return s.deficits[nodeID]
}

func (s *Scheduler) updateDeficit(nodeID identity.ID, d float64) {
	deficit := s.deficits[nodeID] + d
	if deficit < 0 {
		// this will never happen and is just here for debugging purposes
		panic("scheduler: deficit is less than 0")
	}
	s.deficits[nodeID] = math.Min(deficit, MaxDeficit)
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
