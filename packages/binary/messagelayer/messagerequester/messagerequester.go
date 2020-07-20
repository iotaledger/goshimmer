package messagerequester

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/events"
)

// MessageRequester takes care of requesting messages.
type MessageRequester struct {
	scheduledRequests map[message.Id]*time.Timer
	options           *Options
	Events            Events

	scheduledRequestsMutex sync.Mutex
}

// New creates a new message requester.
func New(optionalOptions ...Option) *MessageRequester {
	return &MessageRequester{
		scheduledRequests: make(map[message.Id]*time.Timer),
		options:           newOptions(optionalOptions),
		Events: Events{
			SendRequest: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(message.Id))(params[0].(message.Id))
			}),
		},
	}
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (requester *MessageRequester) StartRequest(id message.Id) {
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	// ignore already scheduled requests
	if _, exists := requester.scheduledRequests[id]; exists {
		return
	}

	// trigger the event and schedule the next request
	// make this atomic to be sure that a successive call of StartRequest does not trigger again
	requester.Events.SendRequest.Trigger(id)
	requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, func() { requester.reRequest(id) })
}

// StopRequest stops requests for the given message to further happen.
func (requester *MessageRequester) StopRequest(id message.Id) {
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	if timer, ok := requester.scheduledRequests[id]; ok {
		timer.Stop()
		delete(requester.scheduledRequests, id)
	}
}

func (requester *MessageRequester) reRequest(id message.Id) {
	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	requester.Events.SendRequest.Trigger(id)

	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := requester.scheduledRequests[id]; exists {
		requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, func() { requester.reRequest(id) })
	}
}
