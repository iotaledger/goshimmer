package ledgerstate

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

// region eventsQueue //////////////////////////////////////////////////////////////////////////////////////////////////

// eventsQueue represents an Event
type eventsQueue struct {
	queuedElements      []*eventsQueueElement
	queuedElementsMutex sync.Mutex
}

// eventsNewQueue returns an empty eventsQueue.
func eventsNewQueue() *eventsQueue {
	return (&eventsQueue{}).clear()
}

// Queue enqueues an Event to be triggered later (using the Trigger function).
func (q *eventsQueue) Queue(event *events.Event, params ...interface{}) {
	q.queuedElementsMutex.Lock()
	defer q.queuedElementsMutex.Unlock()

	q.queuedElements = append(q.queuedElements, &eventsQueueElement{
		event:  event,
		params: params,
	})
}

// Trigger triggers all queued Events and empties the eventsQueue.
func (q *eventsQueue) Trigger() {
	q.queuedElementsMutex.Lock()
	defer q.queuedElementsMutex.Unlock()

	for _, queuedElement := range q.queuedElements {
		queuedElement.event.Trigger(queuedElement.params...)
	}
	q.clear()
}

// Clear removes all elements from the eventsQueue.
func (q *eventsQueue) Clear() {
	q.queuedElementsMutex.Lock()
	defer q.queuedElementsMutex.Unlock()

	q.clear()
}

// clear removes all elements from the eventsQueue without locking it.
func (q *eventsQueue) clear() *eventsQueue {
	q.queuedElements = make([]*eventsQueueElement, 0)

	return q
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region eventsQueueElement ///////////////////////////////////////////////////////////////////////////////////////////

// eventsQueueElement is a struct that holds the information about a triggered Event.
type eventsQueueElement struct {
	event  *events.Event
	params []interface{}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
