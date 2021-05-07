package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/cockroachdb/errors"
)

const (
	// MaxDeficit is the maximum deficit, i.e. max bytes that can be scheduled without waiting; >= maxMessageSize
	MaxDeficit = MaxMessageSize
	// MinMana is the minimum mana require to be able to issue a message.
	// TODO: what is a good value? Would something > MaxMessageSize / 1000 be possible
	MinMana = 1e-6
)

// ErrNotRunning is returned when a message is submitted when the scheduler has not been started
var ErrNotRunning = errors.New("scheduler is not running")

var (
	// TODO: currently the worker pool just serves as a buffer to allow submitting messages to a not running scheduler; investigate why this is necessary.
	submitWorkerCount     = 1
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
	Events *SchedulerEvents

	tangle  *Tangle
	self    identity.ID
	ticker  *time.Ticker
	running typeutils.AtomicBool

	mu       sync.Mutex
	buffer   *schedulerutils.BufferQueue
	deficits map[identity.ID]float64

	closures struct {
		messageSolid     *events.Closure
		messageInvalid   *events.Closure
		messageDiscarded *events.Closure
	}

	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
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
		self:           tangle.Options.Identity.ID(),
		tangle:         tangle,
		buffer:         schedulerutils.NewBufferQueue(),
		deficits:       make(map[identity.ID]float64),
		shutdownSignal: make(chan struct{}),
	}
	scheduler.closures.messageSolid = events.NewClosure(scheduler.messageSolidHandler)
	scheduler.closures.messageInvalid = events.NewClosure(scheduler.messageInvalidHandler)
	scheduler.closures.messageDiscarded = events.NewClosure(scheduler.messageDiscardedHandler)

	submitWorkerPool = workerpool.New(func(task workerpool.Task) {
		if err := scheduler.SubmitAndReady(task.Param(0).(MessageID)); err != nil {
			scheduler.tangle.Events.Error.Trigger(errors.Errorf("failed to submit to scheduler: %w", err))
		}
		task.Return(nil)
	}, workerpool.WorkerCount(submitWorkerCount), workerpool.QueueSize(submitWorkerQueueSize))

	return scheduler
}

func (s *Scheduler) messageSolidHandler(id MessageID) {
	submitWorkerPool.TrySubmit(id)
}

func (s *Scheduler) messageInvalidHandler(id MessageID) {
	s.tangle.Events.Error.Trigger(errors.Errorf("invalid message in scheduler: %x", id))
}

func (s *Scheduler) messageDiscardedHandler(id MessageID) {
	s.tangle.Storage.DeleteMessage(id)
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	// create the ticker here to assure that SetRate never hits an uninitialized ticker
	s.ticker = time.NewTicker(s.tangle.Options.SchedulerParams.Rate)
	// start the main loop
	go s.mainLoop()

	// start schedule queued messages
	submitWorkerPool.Start()

	s.running.Set()
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		// lock the scheduler to make sure that any Submit() has been finished
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running.UnSet()
		close(s.shutdownSignal)
	})
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	s.tangle.Solidifier.Events.MessageSolid.Attach(s.closures.messageSolid)
	s.tangle.Events.MessageInvalid.Attach(s.closures.messageInvalid)
	// TODO: if possible this should be moved to tangle.Storage setup
	s.tangle.Scheduler.Events.MessageDiscarded.Attach(s.closures.messageDiscarded)
}

// Detach detaches the scheduler from the tangle events.
func (s *Scheduler) Detach() {
	s.tangle.Solidifier.Events.MessageSolid.Detach(s.closures.messageSolid)
	s.tangle.Events.MessageInvalid.Detach(s.closures.messageInvalid)
	s.tangle.Scheduler.Events.MessageDiscarded.Detach(s.closures.messageDiscarded)
}

// SubmitAndReady submits the message to the scheduler and marks it ready right away.
func (s *Scheduler) SubmitAndReady(messageID MessageID) error {
	// submit the message to the scheduler and marks it ready right away
	if err := s.Submit(messageID); err != nil {
		return err
	}
	if err := s.Ready(messageID); err != nil {
		return err
	}
	return nil
}

// SetRate sets the rate of the scheduler.
// SetRate must only be called after the scheduler has been started.
func (s *Scheduler) SetRate(rate time.Duration) {
	s.tangle.Options.SchedulerParams.Rate = rate
	// only update the ticker when the scheduler is running
	if s.running.IsSet() {
		s.ticker.Reset(rate)
	}
}

// Submit submits a message to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(messageID) has been called.
func (s *Scheduler) Submit(messageID MessageID) (err error) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.running.IsSet() {
			err = ErrNotRunning
			return
		}

		nodeID := identity.NewID(message.IssuerPublicKey())
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(nodeID)
		if mana < MinMana {
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
		err = errors.Errorf("failed to get message '%x' from storage", messageID)
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
		err = errors.Errorf("failed to get message '%x' from storage", messageID)
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
		err = errors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
}

// Clear removes all submitted messages (ready or not) from the scheduler.
// The MessageDiscarded event is triggered for each of these messages.
func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for q := s.buffer.Current(); q != nil; q = s.buffer.Next() {
		s.buffer.RemoveNode(q.NodeID())
		for _, id := range q.IDs() {
			s.Events.MessageDiscarded.Trigger(MessageID(id))
		}
	}
}

func (s *Scheduler) schedule() *Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.buffer.Current()
	// no messages submitted
	if start == nil {
		return nil
	}

	readyNode := false
	now := time.Now()
	for q := start; ; {
		msg := q.Front()
		// a message can be scheduled, if it is ready and its issuing time is not in the future
		hasReady := msg != nil && !now.Before(msg.IssuingTime())
		// if the current node has enough deficit and the first message in its outbox is valid, we are done
		if hasReady && s.getDeficit(q.NodeID()) >= float64(len(msg.Bytes())) {
			break
		}
		// otherwise increase its deficit
		mana := s.tangle.Options.SchedulerParams.AccessManaRetrieveFunc(q.NodeID())
		// assure that the deficit increase is never too small
		s.updateDeficit(q.NodeID(), math.Max(mana, MinMana))
		if hasReady {
			readyNode = true
		}

		// progress tho the next node that has ready messages
		q = s.buffer.Next()
		// if we reached the first node again without seeing any eligible nodes, break to prevent infinite loops
		if !readyNode && q == start {
			return nil
		}
	}

	// remove the message from the buffer and adjust node's deficit
	msg := s.buffer.PopFront()
	nodeID := identity.NewID(msg.IssuerPublicKey())
	s.updateDeficit(nodeID, -float64(len(msg.Bytes())))

	return msg.(*Message)
}

// mainLoop periodically triggers the scheduling of ready messages.
func (s *Scheduler) mainLoop() {
	defer s.ticker.Stop()

loop:
	for {
		select {
		// every rate time units
		case <-s.ticker.C:
			// TODO: pause the ticker, if there are no ready messages
			if msg := s.schedule(); msg != nil {
				s.Events.MessageScheduled.Trigger(msg.ID())
			}

		// on close, exit the loop
		case <-s.shutdownSignal:
			break loop
		}
	}

	// remove all unscheduled messages
	s.Clear()
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
