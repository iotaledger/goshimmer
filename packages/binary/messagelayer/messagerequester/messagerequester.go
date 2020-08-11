package messagerequester

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/events"
)

const messageExistCheckThreshold = 10

// MessageRequester takes care of requesting messages.
type MessageRequester struct {
	scheduledRequests map[message.ID]*time.Timer
	options           *Options
	messageExistsFunc MessageExistsFunc
	Events            Events

	scheduledRequestsMutex sync.RWMutex
}

// MessageExistsFunc is a function that tells if a message exists.
type MessageExistsFunc func(messageId message.ID) bool

func createReRequest(requester *MessageRequester, msgID message.ID, count int) func() {
	return func() { requester.reRequest(msgID, count) }
}

// New creates a new message requester.
func New(messageExists MessageExistsFunc, missingMessages []message.ID, optionalOptions ...Option) *MessageRequester {
	requester := &MessageRequester{
		scheduledRequests: make(map[message.ID]*time.Timer),
		options:           newOptions(optionalOptions),
		messageExistsFunc: messageExists,
		Events: Events{
			SendRequest: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(message.ID))(params[0].(message.ID))
			}),
			MissingMessageAppeared: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(message.ID))(params[0].(message.ID))
			}),
		},
	}

	// add requests for all missing messages
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	for _, id := range missingMessages {
		requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, createReRequest(requester, id, 0))
	}

	return requester
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (requester *MessageRequester) StartRequest(id message.ID) {
	requester.scheduledRequestsMutex.Lock()

	// ignore already scheduled requests
	if _, exists := requester.scheduledRequests[id]; exists {
		requester.scheduledRequestsMutex.Unlock()
		return
	}

	// schedule the next request and trigger the event
	requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, createReRequest(requester, id, 0))
	requester.scheduledRequestsMutex.Unlock()
	requester.Events.SendRequest.Trigger(id)
}

// StopRequest stops requests for the given message to further happen.
func (requester *MessageRequester) StopRequest(id message.ID) {
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	if timer, ok := requester.scheduledRequests[id]; ok {
		timer.Stop()
		delete(requester.scheduledRequests, id)
	}
}

func (requester *MessageRequester) reRequest(id message.ID, count int) {
	requester.Events.SendRequest.Trigger(id)

	count++
	stopRequest := count > messageExistCheckThreshold && requester.messageExistsFunc(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := requester.scheduledRequests[id]; exists {
		if stopRequest {
			// if found message tangle: stop request and delete from missingMessageStorage (via event)
			delete(requester.scheduledRequests, id)
			requester.Events.MissingMessageAppeared.Trigger(id)
			return
		}
		requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, createReRequest(requester, id, count))
		return
	}
}

// RequestQueueSize returns the number of scheduled message requests.
func (requester *MessageRequester) RequestQueueSize() int {
	requester.scheduledRequestsMutex.RLock()
	defer requester.scheduledRequestsMutex.RUnlock()
	return len(requester.scheduledRequests)
}
