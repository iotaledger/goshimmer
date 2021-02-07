package tangle

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"
)

var (
	capacity   = 1000
	numWorkers = runtime.NumCPU() * 4
)

type Dependencies map[MessageID][]*Message

type Scheduler struct {
	tangle           *Tangle
	inbox            chan *Message
	timeBasedBuffer  *TimeMessageQueue
	dependenciesMap  Dependencies
	outboxWorkerPool async.WorkerPool
}

func NewScheduler(tangle *Tangle) (scheduler *Scheduler) {
	scheduler = &Scheduler{
		tangle:          tangle,
		inbox:           make(chan *Message, capacity),
		timeBasedBuffer: NewTimeMessageQueue(capacity),
		dependenciesMap: make(Dependencies),
	}
	scheduler.outboxWorkerPool.Tune(numWorkers)
	return
}

func (s *Scheduler) Close() {
	close(s.inbox)
	s.outboxWorkerPool.ShutdownGracefully()
	s.timeBasedBuffer.Stop()
}

func (s *Scheduler) Run() {
	s.timeBasedBuffer.Start()
	for {
		select {
		case message := <-s.inbox:
			s.timeBasedBuffer.Add(message)
		case message := <-s.timeBasedBuffer.C:
			deps := s.dependencies(message)
			if len(deps) != 0 {
				for _, parent := range deps {
					if _, exist := s.dependenciesMap[parent]; !exist {
						s.dependenciesMap[parent] = make([]*Message, 0)
					}
					s.dependenciesMap[parent] = append(s.dependenciesMap[parent], message)
				}
				break
			}
			s.outboxWorkerPool.Submit()
		}
	}
}

type timeIssuanceSortedList []*Message
type TimeMessageQueue struct {
	timeIssuanceSortedList
	sync.Mutex
	C     chan *Message
	timer time.Timer
	close chan interface{}
}

func NewTimeMessageQueue(capacity int) *TimeMessageQueue {
	return &TimeMessageQueue{
		C: make(chan *Message, capacity),
	}
}

func (t *TimeMessageQueue) Start() {
	go func() {
		var msg *Message
		for {
			select {
			case <-t.close:
				return
			case <-t.timer.C:
				msg, t.timeIssuanceSortedList = t.timeIssuanceSortedList.pop()
				t.C <- msg
			}
		}
	}()
}

func (t *TimeMessageQueue) Add(message *Message) {
	t.Lock()
	defer t.Unlock()

	if len(t.timeIssuanceSortedList) > 0 && message.IssuingTime().Before(t.timeIssuanceSortedList[0].IssuingTime()) {
		t.timer.Stop()
	}

	t.timeIssuanceSortedList = t.timeIssuanceSortedList.insert(message)

	t.timer.Reset(time.Until(message.IssuingTime()))
}

func (t *TimeMessageQueue) Stop() {
	t.timer.Stop()
	close(t.close)
}

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
		return
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

func (s *Scheduler) dependencies(message *Message) (dependencies MessageIDs) {
	message.ForEachParent(func(parent Parent) {
		s.tangle.Storage.MessageMetadata(parent.ID).Consume(func(parentMetadata *MessageMetadata) {
			if !parentMetadata.IsBooked() {
				dependencies = append(dependencies, parent.ID)
			}
		})
	})
	return dependencies
}

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
