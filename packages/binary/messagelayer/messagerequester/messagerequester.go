package messagerequester

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// MessageRequester takes care of requesting messages.
type MessageRequester struct {
	scheduledRequests map[message.Id]*time.Timer
	requestWorker     async.NonBlockingWorkerPool
	options           *Options
	Events            Events

	scheduledRequestsMutex sync.RWMutex
}

// New creates a new message requester.
func New(optionalOptions ...Option) *MessageRequester {
	requester := &MessageRequester{
		scheduledRequests: make(map[message.Id]*time.Timer),
		options:           newOptions(optionalOptions),
		Events: Events{
			SendRequest: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(message.Id))(params[0].(message.Id))
			}),
		},
	}

	requester.requestWorker.Tune(requester.options.workerCount)
	return requester
}

// ScheduleRequest schedules a request for the given message.
func (requester *MessageRequester) ScheduleRequest(messageId message.Id) {
	var retryRequest func(bool)
	retryRequest = func(initialRequest bool) {
		requester.requestWorker.Submit(func() {
			requester.scheduledRequestsMutex.RLock()
			if _, requestExists := requester.scheduledRequests[messageId]; !initialRequest && !requestExists {
				requester.scheduledRequestsMutex.RUnlock()
				return
			}
			requester.scheduledRequestsMutex.RUnlock()

			requester.Events.SendRequest.Trigger(messageId)

			requester.scheduledRequestsMutex.Lock()
			requester.scheduledRequests[messageId] = time.AfterFunc(requester.options.retryInterval, func() { retryRequest(false) })
			requester.scheduledRequestsMutex.Unlock()
		})
	}

	retryRequest(true)
}

// StopRequest stops requests for the given message to further happen.
func (requester *MessageRequester) StopRequest(messageId message.Id) {
	requester.scheduledRequestsMutex.RLock()
	if timer, timerExists := requester.scheduledRequests[messageId]; timerExists {
		requester.scheduledRequestsMutex.RUnlock()

		timer.Stop()

		requester.scheduledRequestsMutex.Lock()
		delete(requester.scheduledRequests, messageId)
		requester.scheduledRequestsMutex.Unlock()
		return
	}
	requester.scheduledRequestsMutex.RUnlock()
}
