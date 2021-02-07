package tangle

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"
)

var (
	capacity   = 1000
	numWorkers = runtime.NumCPU() * 4
)

// SchedulerParentPriorityMap maps parentIDs with their children messages.
type SchedulerParentPriorityMap map[MessageID][]*messageToSchedule

type messageToSchedule struct {
	message   *Message
	scheduled bool
}

// Scheduler implements the scheduler.
type Scheduler struct {
	Events *SchedulerEvents

	onMessageSolid  *events.Closure
	onMessageBooked *events.Closure
	tangle          *Tangle

	inbox            chan *Message
	timeQueue        *TimeMessageQueue
	parentsMap       SchedulerParentPriorityMap
	messagesBooked   chan MessageID
	outboxWorkerPool async.WorkerPool
	close            chan interface{}
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) (scheduler *Scheduler) {
	scheduler = &Scheduler{
		Events: &SchedulerEvents{
			MessageScheduled: events.NewEvent(messageIDEventHandler),
		},
		tangle:         tangle,
		inbox:          make(chan *Message, capacity),
		timeQueue:      NewTimeMessageQueue(capacity),
		parentsMap:     make(SchedulerParentPriorityMap),
		messagesBooked: make(chan MessageID, capacity),
		close:          make(chan interface{}),
	}
	scheduler.outboxWorkerPool.Tune(numWorkers)

	// setup scheduler flow
	scheduler.onMessageSolid = events.NewClosure(func(messageID MessageID) {
		scheduler.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			scheduler.inbox <- message
		})
	})
	scheduler.tangle.Solidifier.Events.MessageSolid.Attach(scheduler.onMessageSolid)

	scheduler.onMessageBooked = events.NewClosure(func(messageID MessageID) {
		scheduler.messagesBooked <- messageID
	})
	scheduler.tangle.Events.MessageBooked.Attach(scheduler.onMessageBooked)

	scheduler.start()

	return
}

// start starts the scheduler.
func (s *Scheduler) start() {
	go func() {
		s.timeQueue.Start()
		for {
			select {

			case message := <-s.inbox:
				if message != nil && message.IssuingTime().After(clock.SyncedTime()) {
					s.timeQueue.Add(message)
					break
				}
				s.trySchedule(message)

			case message := <-s.timeQueue.C:
				s.trySchedule(message)

			case messageID := <-s.messagesBooked:
				for _, child := range s.parentsMap[messageID] {
					if s.messageReady(child.message) && !child.scheduled {
						child.scheduled = true
						s.schedule(child.message.ID())
					}
				}
				delete(s.parentsMap, messageID)

			case <-s.close:
				return
			}
		}
	}()
}

// Stop stops the scheduler and terminates its goroutines and timers.
func (s *Scheduler) Stop() {
	close(s.close)
	close(s.inbox)
	s.tangle.Solidifier.Events.MessageSolid.Detach(s.onMessageSolid)
	s.tangle.Events.MessageBooked.Detach(s.onMessageBooked)

	s.outboxWorkerPool.ShutdownGracefully()
	s.timeQueue.Stop()
}

func (s *Scheduler) schedule(messageID MessageID) {
	s.outboxWorkerPool.Submit(func() {
		s.Events.MessageScheduled.Trigger(messageID)
	})
}

func (s *Scheduler) trySchedule(message *Message) {
	if message == nil {
		return
	}

	parents := s.priorities(message)
	if len(parents) > 0 {
		child := &messageToSchedule{message: message}
		for _, parent := range parents {
			if _, exist := s.parentsMap[parent]; !exist {
				s.parentsMap[parent] = make([]*messageToSchedule, 0)
			}
			s.parentsMap[parent] = append(s.parentsMap[parent], child)
		}
		return
	}
	s.schedule(message.ID())
}

func (s *Scheduler) messageReady(message *Message) (ready bool) {
	return len(s.priorities(message)) == 0
}

func (s *Scheduler) priorities(message *Message) (parents MessageIDs) {
	message.ForEachParent(func(parent Parent) {
		s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
			if !messageMetadata.IsBooked() {
				parents = append(parents, parent.ID)
			}
		})

	})
	return parents
}

// region TimeMessageQueue /////////////////////////////////////////////////////////////////////////////////////////////

// TimeMessageQueue is a time-based ordered queue.
type TimeMessageQueue struct {
	timeIssuanceSortedList
	sync.Mutex
	C     chan *Message
	timer *time.Timer
	close chan interface{}
}

// NewTimeMessageQueue returns a new TimeMessageQueue.
func NewTimeMessageQueue(capacity int) *TimeMessageQueue {
	return &TimeMessageQueue{
		timer: time.NewTimer(0),
		C:     make(chan *Message, capacity),
		close: make(chan interface{}),
	}
}

// Start starts the TimeMessageQueue.
func (t *TimeMessageQueue) Start() {
	go func() {
		var msg *Message
		for {
			select {
			case <-t.close:
				return
			case <-t.timer.C:
				msg, t.timeIssuanceSortedList = t.timeIssuanceSortedList.pop()
				if msg != nil {
					t.C <- msg
				}
			}
		}
	}()
}

// Stop stops the TimeMessageQueue.
func (t *TimeMessageQueue) Stop() {
	t.timer.Stop()
	close(t.close)
}

// Add adds a message to the TimeMessageQueue.
func (t *TimeMessageQueue) Add(message *Message) {
	t.Lock()
	defer t.Unlock()

	if len(t.timeIssuanceSortedList) > 0 && message.IssuingTime().Before(t.timeIssuanceSortedList[0].IssuingTime()) {
		t.timer.Stop()
	}

	t.timeIssuanceSortedList = t.timeIssuanceSortedList.insert(message)

	t.timer.Reset(time.Until(message.IssuingTime()))
}

// region TimeMessageQueue /////////////////////////////////////////////////////////////////////////////////////////////

type timeIssuanceSortedList []*Message

func (t timeIssuanceSortedList) insert(message *Message) (list timeIssuanceSortedList) {
	position := -1
	for i, m := range t {
		if message.IssuingTime().Before(m.IssuingTime()) {
			position = i
			break
		}
	}
	switch position {
	case 0:
		list = append(timeIssuanceSortedList{message}, t[:]...)
	case -1:
		list = append(t, message)
	default:
		list = append(t[:position], append(timeIssuanceSortedList{message}, t[position:]...)...)
	}
	return
}

func (t timeIssuanceSortedList) pop() (message *Message, list timeIssuanceSortedList) {
	if len(t) == 0 {
		return nil, t
	}

	message = t[0]
	list = append(timeIssuanceSortedList{}, t[1:]...)

	return
}

func (t timeIssuanceSortedList) String() (s string) {
	for i, m := range t {
		s += fmt.Sprintf("%d - %v\n", i, m.IssuingTime())
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
