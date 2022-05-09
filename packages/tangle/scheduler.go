package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/typeutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"
)

const (
	// MaxDeficit is the maximum cap for accumulated deficit, i.e. max bytes that can be scheduled without waiting.
	// It must be >= MaxMessageSize.
	MaxDeficit = MaxMessageSize
	// MinMana is the minimum amount of Mana needed to issue messages.
	// MaxMessageSize / MinMana is also the upper bound of iterations inside one schedule call, as such it should not be too small.
	MinMana float64 = 1.0
)

// ErrNotRunning is returned when a message is submitted when the scheduler has been stopped.
var ErrNotRunning = errors.New("scheduler stopped")

// SchedulerParams defines the scheduler config parameters.
type SchedulerParams struct {
	MaxBufferSize                     int
	Rate                              time.Duration
	AccessManaRetrieveFunc            func(identity.ID) float64
	TotalAccessManaRetrieveFunc       func() float64
	AccessManaMapRetrieverFunc        func() map[identity.ID]float64
	ConfirmedMessageScheduleThreshold time.Duration
}

// Scheduler is a Tangle component that takes care of scheduling the messages that shall be booked.
type Scheduler struct {
	Events *SchedulerEvents

	tangle                *Tangle
	ticker                *time.Ticker
	started               typeutils.AtomicBool
	stopped               typeutils.AtomicBool
	accessManaCache       *schedulerutils.AccessManaCache
	mu                    sync.RWMutex
	buffer                *schedulerutils.BufferQueue
	deficits              map[identity.ID]float64
	rate                  *atomic.Duration
	confirmedMsgThreshold time.Duration
	shutdownSignal        chan struct{}
	shutdownOnce          sync.Once
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) *Scheduler {
	if tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc == nil || tangle.Options.SchedulerParams.AccessManaRetrieveFunc == nil || tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc == nil {
		panic("scheduler: the option AccessManaMapRetrieverFunc and AccessManaRetriever and TotalAccessManaRetriever must be defined so that AccessMana can be determined in scheduler")
	}

	// maximum buffer size (in bytes)
	maxBuffer := tangle.Options.SchedulerParams.MaxBufferSize

	// threshold after which confirmed messages are not scheduled
	confirmedMessageScheduleThreshold := tangle.Options.SchedulerParams.ConfirmedMessageScheduleThreshold

	// maximum access mana-scaled inbox length
	maxQueue := float64(maxBuffer) / float64(tangle.LedgerState.TotalSupply())

	accessManaCache := schedulerutils.NewAccessManaCache(tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc, MinMana)

	return &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(MessageIDCaller),
			MessageDiscarded: events.NewEvent(MessageIDCaller),
			MessageSkipped:   events.NewEvent(MessageIDCaller),
			NodeBlacklisted:  events.NewEvent(NodeIDCaller),
			Error:            events.NewEvent(events.ErrorCaller),
		},
		tangle:                tangle,
		accessManaCache:       accessManaCache,
		rate:                  atomic.NewDuration(tangle.Options.SchedulerParams.Rate),
		ticker:                time.NewTicker(tangle.Options.SchedulerParams.Rate),
		buffer:                schedulerutils.NewBufferQueue(maxBuffer, maxQueue),
		confirmedMsgThreshold: confirmedMessageScheduleThreshold,
		deficits:              make(map[identity.ID]float64),
		shutdownSignal:        make(chan struct{}),
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.started.Set()
	// start the main loop
	go s.mainLoop()
}

// Running returns true if the scheduler has started.
func (s *Scheduler) Running() bool {
	return s.started.IsSet()
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		// lock the scheduler to make sure that any Submit() has been finished
		s.mu.Lock()
		defer s.mu.Unlock()
		s.stopped.Set()
		close(s.shutdownSignal)
	})
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	// pass booked messages to the scheduler
	s.tangle.ApprovalWeightManager.Events.MessageProcessed.Attach(events.NewClosure(func(messageID MessageID) {
		if err := s.Submit(messageID); err != nil {
			if !errors.Is(err, schedulerutils.ErrInsufficientMana) {
				s.Events.Error.Trigger(errors.Errorf("failed to submit to scheduler: %w", err))
			}
		}
		s.tryReady(messageID)
	}))

	s.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(s.updateApprovers))

	onMessageConfirmed := func(messageID MessageID) {
		var scheduled bool
		s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			scheduled = messageMetadata.Scheduled()
		})
		if scheduled {
			return
		}
		s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			if clock.Since(message.IssuingTime()) > s.confirmedMsgThreshold {
				err := s.Unsubmit(messageID)
				if err != nil {
					s.Events.Error.Trigger(errors.Errorf("failed to unsubmit confirmed message from scheduler: %w", err))
				}
				s.Events.MessageSkipped.Trigger(messageID)
			}
		})
		s.updateApprovers(messageID)
	}
	s.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(events.NewClosure(onMessageConfirmed))

	s.Start()
}

// SetRate sets the rate of the scheduler.
func (s *Scheduler) SetRate(rate time.Duration) {
	// only update the ticker when the scheduler is running
	if !s.stopped.IsSet() {
		s.ticker.Reset(rate)
		s.rate.Store(rate)
	}
}

// Rate gets the rate of the scheduler.
func (s *Scheduler) Rate() time.Duration {
	return s.rate.Load()
}

// NodeQueueSize returns the size of the nodeIDs queue.
func (s *Scheduler) NodeQueueSize(nodeID identity.ID) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodeQueue := s.buffer.NodeQueue(nodeID)
	if nodeQueue == nil {
		return 0
	}
	return nodeQueue.Size()
}

// NodeQueueSizes returns the size for each node queue.
func (s *Scheduler) NodeQueueSizes() map[identity.ID]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodeQueueSizes := make(map[identity.ID]int)
	for _, nodeID := range s.buffer.NodeIDs() {
		size := s.buffer.NodeQueue(nodeID).Size()
		nodeQueueSizes[nodeID] = size
	}
	return nodeQueueSizes
}

// MaxBufferSize returns the max size of the buffer.
func (s *Scheduler) MaxBufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffer.MaxSize()
}

// BufferSize returns the size of the buffer.
func (s *Scheduler) BufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffer.Size()
}

// ReadyMessagesCount returns the size buffer.
func (s *Scheduler) ReadyMessagesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffer.ReadyMessagesCount()
}

// TotalMessagesCount returns the size buffer.
func (s *Scheduler) TotalMessagesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buffer.TotalMessagesCount()
}

// AccessManaCache returns the object which caches access mana values.
func (s *Scheduler) AccessManaCache() *schedulerutils.AccessManaCache {
	return s.accessManaCache
}

// Submit submits a message to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(messageID) has been called.
func (s *Scheduler) Submit(messageID MessageID) (err error) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()
		err = s.submit(message)
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

		s.unsubmit(message)
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

		s.ready(message)
	}) {
		err = errors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
}

// SubmitAndReady submits the message to the scheduler and marks it ready right away.
func (s *Scheduler) SubmitAndReady(messageID MessageID) (err error) {
	if !s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.mu.Lock()
		defer s.mu.Unlock()

		err = s.submit(message)
		if err == nil {
			s.ready(message)
		}
	}) {
		err = errors.Errorf("failed to get message '%x' from storage", messageID)
	}
	return err
}

// GetManaFromCache allows you to get the cached mana for a node ID. This is exposed for analytics purposes.
func (s *Scheduler) GetManaFromCache(nodeID identity.ID) float64 {
	return s.accessManaCache.GetCachedMana(nodeID)
}

// Clear removes all submitted messages (ready or not) from the scheduler.
// The MessageDiscarded event is triggered for each of these messages.
func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for q := s.buffer.Current(); q != nil; q = s.buffer.Next() {
		s.buffer.RemoveNode(q.NodeID())
		for _, id := range q.IDs() {
			messageID := MessageID(id)
			s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				messageMetadata.SetDiscardedTime(clock.SyncedTime())
			})
			s.Events.MessageDiscarded.Trigger(messageID)
		}
	}
}

// isEligible returns true if the given messageID has either been scheduled or confirmed.
func (s *Scheduler) isEligible(messageID MessageID) (eligible bool) {
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = messageMetadata.Scheduled() ||
			s.tangle.ConfirmationOracle.IsMessageConfirmed(messageID)
	})
	return
}

// isReady returns true if the given messageID's parents are eligible.
func (s *Scheduler) isReady(messageID MessageID) (ready bool) {
	ready = true
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		message.ForEachParent(func(parent Parent) {
			if !s.isEligible(parent.ID) { // parents are not eligible
				ready = false
				return
			}
		})
	})

	return
}

// tryReady tries to set the given message as ready.
func (s *Scheduler) tryReady(messageID MessageID) {
	if s.isReady(messageID) {
		if err := s.Ready(messageID); err != nil {
			s.Events.Error.Trigger(errors.Errorf("failed to mark %s as ready: %w", messageID, err))
		}
	}
}

// updateApprovers iterates over the direct approvers of the given messageID and
// tries to mark them as ready.
func (s *Scheduler) updateApprovers(messageID MessageID) {
	s.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		s.tryReady(approver.ApproverMessageID())
	})
}

func (s *Scheduler) submit(message *Message) error {
	if s.stopped.IsSet() {
		return ErrNotRunning
	}

	s.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// shortly before submitting we set the queued time
		messageMetadata.SetQueuedTime(clock.SyncedTime())
	})
	// when removing the zero mana node solution, check if nodes have MinMana here
	droppedMessageIDs := s.buffer.Submit(message, s.accessManaCache.GetCachedMana)
	for _, droppedMsgID := range droppedMessageIDs {
		s.tangle.Storage.MessageMetadata(MessageID(droppedMsgID)).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetDiscardedTime(clock.SyncedTime())
		})
		s.Events.MessageDiscarded.Trigger(MessageID(droppedMsgID))
	}
	return nil
}

func (s *Scheduler) unsubmit(message *Message) {
	s.buffer.Unsubmit(message)
}

func (s *Scheduler) ready(message *Message) {
	s.buffer.Ready(message)
}

func (s *Scheduler) schedule() *Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updateActiveNodesList(s.accessManaCache.RawAccessManaVector())

	start := s.buffer.Current()
	// no messages submitted
	if start == nil {
		return nil
	}

	var schedulingNode *schedulerutils.NodeQueue
	rounds := math.MaxInt32
	for q := start; ; {
		msg := q.Front()
		// a message can be scheduled, if it is ready
		// (its issuing time is not in the future and all of its parents are eligible).
		// while loop to skip all the confirmed messages
		for msg != nil && !clock.SyncedTime().Before(msg.IssuingTime()) {
			msgID, _, err := MessageIDFromBytes(msg.IDBytes())
			if err != nil {
				panic("MessageID could not be parsed!")
			}
			if s.tangle.ConfirmationOracle.IsMessageConfirmed(msgID) && clock.Since(msg.IssuingTime()) > s.confirmedMsgThreshold {
				// if a message is confirmed, and issued some time ago, don't schedule it and take the next one from the queue
				// do we want to mark those messages somehow for debugging?
				s.Events.MessageSkipped.Trigger(msgID)
				s.buffer.PopFront()
				msg = q.Front()
			} else {
				// compute how often the deficit needs to be incremented until the message can be scheduled
				remainingDeficit := math.Dim(float64(msg.Size()), s.getDeficit(q.NodeID()))
				nodeMana := s.accessManaCache.GetCachedMana(q.NodeID())
				// find the first node that will be allowed to schedule a message
				if r := int(math.Ceil(remainingDeficit / nodeMana)); r < rounds {
					rounds = r
					schedulingNode = q
				}
				break
			}
		}

		q = s.buffer.Next()
		if q == start {
			break
		}
	}

	// if there is no node with a ready message, we cannot schedule anything
	if schedulingNode == nil {
		return nil
	}

	if rounds > 0 {
		// increment every node's deficit for the required number of rounds
		for q := start; ; {
			s.updateDeficit(q.NodeID(), float64(rounds)*s.accessManaCache.GetCachedMana(q.NodeID()))

			q = s.buffer.Next()
			if q == start {
				break
			}
		}
	}

	// increment the deficit for all nodes before schedulingNode one more time
	for q := start; q != schedulingNode; q = s.buffer.Next() {
		s.updateDeficit(q.NodeID(), s.accessManaCache.GetCachedMana(q.NodeID()))
	}

	// remove the message from the buffer and adjust node's deficit
	msg := s.buffer.PopFront()
	nodeID := identity.NewID(msg.IssuerPublicKey())
	s.updateDeficit(nodeID, -float64(msg.Size()))

	return msg.(*Message)
}

func (s *Scheduler) updateActiveNodesList(manaCache map[identity.ID]float64) {
	currentNode := s.buffer.Current()
	// use counter to avoid infinite loop in case the start element is removed
	activeNodes := s.buffer.NumActiveNodes()
	// remove nodes that don't have mana and have empty queue
	// this allows nodes with zero mana to issue messages, however those nodes will only accumulate their deficit
	// when there are messages in the node's queue
	for i := 0; i < activeNodes; i++ {
		if nodeMana, exists := manaCache[currentNode.NodeID()]; (!exists || nodeMana < MinMana) && currentNode.Size() == 0 {
			s.buffer.RemoveNode(currentNode.NodeID())
			delete(s.deficits, currentNode.NodeID())
			currentNode = s.buffer.Current()
		} else {
			currentNode = s.buffer.Next()
		}
	}

	// update list of active nodes with accumulating deficit
	for nodeID, nodeMana := range manaCache {
		if nodeMana < MinMana {
			continue
		}
		if _, exists := s.deficits[nodeID]; !exists {
			s.deficits[nodeID] = 0
			s.buffer.InsertNode(nodeID)
		}
	}
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
				s.tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
					if messageMetadata.SetScheduled(true) {
						s.Events.MessageScheduled.Trigger(msg.ID())
					}
				})
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
	// MessageDiscarded is triggered when a message is removed from the longest mana-scaled queue when the buffer is full.
	MessageDiscarded *events.Event
	// MessageSkipped is triggered when a message is confirmed before it's scheduled, and is skipped by the scheduler.
	MessageSkipped  *events.Event
	NodeBlacklisted *events.Event
	Error           *events.Event
}

// NodeIDCaller is the caller function for events that hand over a NodeID.
func NodeIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(identity.ID))(params[0].(identity.ID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
