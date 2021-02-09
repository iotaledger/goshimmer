package tangle

import (
	"fmt"
	"runtime"
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
type SchedulerParentPriorityMap map[MessageID][]*MessageToSchedule

// MessageToSchedule contains the relevant information to schedule a message.
type MessageToSchedule struct {
	ID          MessageID
	parents     []MessageID
	issuingTime time.Time
	scheduled   bool
}

// Scheduler implements the scheduler.
type Scheduler struct {
	Events *SchedulerEvents

	tangle *Tangle

	inbox            chan *MessageToSchedule
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
		inbox:          make(chan *MessageToSchedule, capacity),
		timeQueue:      NewTimeMessageQueue(capacity),
		parentsMap:     make(SchedulerParentPriorityMap),
		messagesBooked: make(chan MessageID, capacity),
		close:          make(chan interface{}),
	}
	scheduler.outboxWorkerPool.Tune(numWorkers)

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	// setup scheduler flow
	onMessageSolid := events.NewClosure(func(messageID MessageID) {
		s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			s.inbox <- &MessageToSchedule{
				ID:          messageID,
				issuingTime: message.IssuingTime(),
				parents:     message.Parents()}
		})
	})
	s.tangle.Solidifier.Events.MessageSolid.Attach(onMessageSolid)

	onMessageBooked := events.NewClosure(func(messageID MessageID) {
		s.messagesBooked <- messageID
	})
	s.tangle.Booker.Events.MessageBooked.Attach(onMessageBooked)

	s.start()
}

// start starts the scheduler.
func (s *Scheduler) start() {
	go func() {
		s.timeQueue.Start()
		for {
			select {

			// read message from the inbox, add them to the timeQueue if their timestamp
			// is in the future or schedule it if its parents have been booked already.
			case message := <-s.inbox:
				if message != nil && message.issuingTime.After(clock.SyncedTime()) {
					s.timeQueue.Add(message)
					break
				}
				s.trySchedule(message)

			// try to schedule messsage that have been waiting and are now current.
			case message := <-s.timeQueue.C:
				s.trySchedule(message)

			// schedule messages that were waiting for their parents to be booked.
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

// Shutdown stops the scheduler and terminates its goroutines and timers.
func (s *Scheduler) Shutdown() {
	close(s.close)
	close(s.inbox)

	s.outboxWorkerPool.ShutdownGracefully()
	s.timeQueue.Stop()
}

func (s *Scheduler) schedule(messageID MessageID) {
	s.outboxWorkerPool.Submit(func() {
		s.Events.MessageScheduled.Trigger(messageID)
	})
}

func (s *Scheduler) trySchedule(message *MessageToSchedule) {
	if message == nil {
		return
	}

	// schedule if all the parents have been booked already.
	parentsToBook := s.parentsToBook(message)
	if len(parentsToBook) == 0 {
		s.schedule(message.ID)
		return
	}

	// append the message to the unbooked parent(s) queue(s).
	for _, parent := range parentsToBook {
		if _, exist := s.parentsMap[parent]; !exist {
			s.parentsMap[parent] = make([]*MessageToSchedule, 0)
		}
		s.parentsMap[parent] = append(s.parentsMap[parent], message)
	}
}

func (s *Scheduler) messageReady(message *MessageToSchedule) (ready bool) {
	return len(s.parentsToBook(message)) == 0
}

func (s *Scheduler) parentsToBook(message *MessageToSchedule) (parents MessageIDs) {
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
	list    timeIssuanceSortedList
	C       chan *MessageToSchedule
	addChan chan *MessageToSchedule
	timer   *time.Timer
	close   chan interface{}
}

// NewTimeMessageQueue returns a new TimeMessageQueue.
func NewTimeMessageQueue(capacity int) *TimeMessageQueue {
	return &TimeMessageQueue{
		list:    make(timeIssuanceSortedList, 0),
		timer:   time.NewTimer(0),
		addChan: make(chan *MessageToSchedule, capacity),
		C:       make(chan *MessageToSchedule, capacity),
		close:   make(chan interface{}),
	}
}

// Start starts the TimeMessageQueue.
func (t *TimeMessageQueue) Start() {
	go func() {
		<-t.timer.C // ignore first timeout
		var msg *MessageToSchedule
		for {
			select {
			case <-t.close:
				return
			case msg = <-t.addChan:
				t.add(msg)
			case <-t.timer.C:
				msg = t.Pop()
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
func (t *TimeMessageQueue) Add(message *MessageToSchedule) {
	t.addChan <- message
}

// add adds a message to the TimeMessageQueue.
func (t *TimeMessageQueue) add(message *MessageToSchedule) {
	if (t.list.insert(message)) == 0 {
		t.timer.Stop()
		t.timer.Reset(time.Until(message.issuingTime))
	}
}

// Pop returns the first message to schedule.
func (t *TimeMessageQueue) Pop() (message *MessageToSchedule) {
	if len(t.list) > 1 {
		t.timer.Reset(time.Until(t.list[1].issuingTime))
	}

	return t.list.pop()
}

// region timeIssuanceSortedList /////////////////////////////////////////////////////////////////////////////////////////////

type timeIssuanceSortedList []*MessageToSchedule

func (t *timeIssuanceSortedList) insert(message *MessageToSchedule) (position int) {
	position = -1
	if len(*t) == 0 {
		position = 0
	}
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
	return
}

func (t *timeIssuanceSortedList) pop() (message *MessageToSchedule) {
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
