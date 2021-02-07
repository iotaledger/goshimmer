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
	ID          MessageID
	parents     []MessageID
	issuingTime time.Time
	scheduled   bool
}

// Scheduler implements the scheduler.
type Scheduler struct {
	Events *SchedulerEvents

	onMessageSolid  *events.Closure
	onMessageBooked *events.Closure
	tangle          *Tangle

	inbox            chan *messageToSchedule
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
		inbox:          make(chan *messageToSchedule, capacity),
		timeQueue:      NewTimeMessageQueue(capacity),
		parentsMap:     make(SchedulerParentPriorityMap),
		messagesBooked: make(chan MessageID, capacity),
		close:          make(chan interface{}),
	}
	scheduler.outboxWorkerPool.Tune(numWorkers)

	// setup scheduler flow
	scheduler.onMessageSolid = events.NewClosure(func(messageID MessageID) {
		scheduler.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			scheduler.inbox <- &messageToSchedule{
				ID:          messageID,
				issuingTime: message.IssuingTime(),
				parents:     message.Parents()}
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
				if message != nil && message.issuingTime.After(clock.SyncedTime()) {
					s.timeQueue.Add(message)
					break
				}
				s.trySchedule(message)

			case message := <-s.timeQueue.C:
				s.trySchedule(message)

			case messageID := <-s.messagesBooked:
				for _, child := range s.parentsMap[messageID] {
					if s.messageReady(child) && !child.scheduled {
						child.scheduled = true
						s.schedule(child.ID)
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

func (s *Scheduler) trySchedule(message *messageToSchedule) {
	if message == nil {
		return
	}

	parents := s.priorities(message)
	if len(parents) > 0 {
		for _, parent := range parents {
			if _, exist := s.parentsMap[parent]; !exist {
				s.parentsMap[parent] = make([]*messageToSchedule, 0)
			}
			s.parentsMap[parent] = append(s.parentsMap[parent], message)
		}
		return
	}
	s.schedule(message.ID)
}

func (s *Scheduler) messageReady(message *messageToSchedule) (ready bool) {
	return len(s.priorities(message)) == 0
}

func (s *Scheduler) priorities(message *messageToSchedule) (parents MessageIDs) {
	for _, parentID := range message.parents {
		s.tangle.Storage.MessageMetadata(parentID).Consume(func(messageMetadata *MessageMetadata) {
			if !messageMetadata.IsBooked() {
				parents = append(parents, parentID)
			}
		})
	}

	return parents
}

// region TimeMessageQueue /////////////////////////////////////////////////////////////////////////////////////////////

// TimeMessageQueue is a time-based ordered queue.
type TimeMessageQueue struct {
	list timeIssuanceSortedList
	sync.Mutex
	C     chan *messageToSchedule
	timer *time.Timer
	close chan interface{}
}

// NewTimeMessageQueue returns a new TimeMessageQueue.
func NewTimeMessageQueue(capacity int) *TimeMessageQueue {
	return &TimeMessageQueue{
		list:  make(timeIssuanceSortedList, 0),
		timer: time.NewTimer(0),
		C:     make(chan *messageToSchedule, capacity),
		close: make(chan interface{}),
	}
}

// Start starts the TimeMessageQueue.
func (t *TimeMessageQueue) Start() {
	go func() {
		var msg *messageToSchedule
		for {
			select {
			case <-t.close:
				return
			case <-t.timer.C:
				t.Lock()
				msg = t.list.pop()
				t.Unlock()
				if msg != nil {
					t.C <- msg
				}
			}
		}
	}()
}

// Stop stops the TimeMessageQueue.
func (t *TimeMessageQueue) Stop() {
	t.Lock()
	defer t.Unlock()
	t.timer.Stop()
	close(t.close)
}

// Add adds a message to the TimeMessageQueue.
func (t *TimeMessageQueue) Add(message *messageToSchedule) {
	t.Lock()
	defer t.Unlock()

	if len(t.list) > 0 && message.issuingTime.Before(t.list[0].issuingTime) {
		t.timer.Stop()
	}

	t.list.insert(message)

	t.timer.Reset(time.Until(message.issuingTime))
}

// region TimeMessageQueue /////////////////////////////////////////////////////////////////////////////////////////////

type timeIssuanceSortedList []*messageToSchedule

func (t *timeIssuanceSortedList) insert(message *messageToSchedule) {
	position := -1
	for i, m := range *t {
		if message.issuingTime.Before(m.issuingTime) {
			position = i
			break
		}
	}
	switch position {
	case 0:
		*t = append(timeIssuanceSortedList{message}, (*t)[:]...)
	case -1:
		*t = append(*t, message)
	default:
		*t = append((*t)[:position], append(timeIssuanceSortedList{message}, (*t)[position:]...)...)
	}
}

func (t *timeIssuanceSortedList) pop() (message *messageToSchedule) {
	if len(*t) == 0 {
		return nil
	}

	message = (*t)[0]
	*t = append(timeIssuanceSortedList{}, (*t)[1:]...)

	return
}

func (t *timeIssuanceSortedList) String() (s string) {
	for i, m := range *t {
		s += fmt.Sprintf("%d - %v\n", i, m.issuingTime)
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
