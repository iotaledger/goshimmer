package eventsqueue

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

// region EventsQueue //////////////////////////////////////////////////////////////////////////////////////////////////

// EventsQueue represents an Event
type EventsQueue struct {
	queuedElements      []*queueElement
	queuedElementsMutex sync.Mutex
}

// New returns an empty EventsQueue.
func New() *EventsQueue {
	return (&EventsQueue{}).clear()
}

// Queue enqueues an Event to be triggered later (using the Trigger function).
func (e *EventsQueue) Queue(event *events.Event, params ...interface{}) {
	e.queuedElementsMutex.Lock()
	defer e.queuedElementsMutex.Unlock()

	e.queuedElements = append(e.queuedElements, &queueElement{
		event:  event,
		params: params,
	})
}

// Trigger triggers all queued Events and empties the EventsQueue.
func (e *EventsQueue) Trigger() {
	e.queuedElementsMutex.Lock()
	defer e.queuedElementsMutex.Unlock()

	for _, queuedElement := range e.queuedElements {
		queuedElement.event.Trigger(queuedElement.params...)
	}
	e.clear()
}

// Clear removes all elements from the EventsQueue.
func (e *EventsQueue) Clear() {
	e.queuedElementsMutex.Lock()
	defer e.queuedElementsMutex.Unlock()

	e.clear()
}

// clear removes all elements from the EventsQueue without locking it.
func (e *EventsQueue) clear() *EventsQueue {
	e.queuedElements = make([]*queueElement, 0)

	return e
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region queueElement /////////////////////////////////////////////////////////////////////////////////////////////////

// queueElement is a struct that holds the information about a triggered Event.
type queueElement struct {
	event  *events.Event
	params []interface{}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
