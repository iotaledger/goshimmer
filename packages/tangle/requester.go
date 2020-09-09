package tangle

import (
	"sync"
	"time"
)

const (
	// DefaultRetryInterval defines the Default Retry Interval of the message requester.
	DefaultRetryInterval = 10 * time.Second
	// the maximum amount of requests before we abort
	maxRequestThreshold = 500
)

// Options holds options for a message requester.
type Options struct {
	retryInterval time.Duration
}

func newOptions(optionalOptions []Option) *Options {
	result := &Options{
		retryInterval: 10 * time.Second,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

// Option is a function which inits an option.
type Option func(*Options)

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval(interval time.Duration) Option {
	return func(args *Options) {
		args.retryInterval = interval
	}
}

// Requester takes care of requesting messages.
type Requester struct {
	scheduledRequests map[MessageID]*time.Timer
	options           *Options
	Events            *RequesterEvents

	scheduledRequestsMutex sync.RWMutex
}

// MessageExistsFunc is a function that tells if a message exists.
type MessageExistsFunc func(messageId MessageID) bool

// NewRequester creates a new message requester.
func NewRequester(missingMessages []MessageID, optionalOptions ...Option) *Requester {
	requester := &Requester{
		scheduledRequests: make(map[MessageID]*time.Timer),
		options:           newOptions(optionalOptions),
		Events:            newRequesterEvents(),
	}

	// add requests for all missing messages
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	for _, id := range missingMessages {
		requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, requester.createReRequest(id, 0))
	}

	return requester
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (requester *Requester) StartRequest(id MessageID) {
	requester.scheduledRequestsMutex.Lock()

	// ignore already scheduled requests
	if _, exists := requester.scheduledRequests[id]; exists {
		requester.scheduledRequestsMutex.Unlock()
		return
	}

	// schedule the next request and trigger the event
	requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, requester.createReRequest(id, 0))
	requester.scheduledRequestsMutex.Unlock()
	requester.Events.SendRequest.Trigger(&SendRequestEvent{ID: id})
}

// StopRequest stops requests for the given message to further happen.
func (requester *Requester) StopRequest(id MessageID) {
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	if timer, ok := requester.scheduledRequests[id]; ok {
		timer.Stop()
		delete(requester.scheduledRequests, id)
	}
}

func (requester *Requester) reRequest(id MessageID, count int) {
	requester.Events.SendRequest.Trigger(&SendRequestEvent{ID: id})

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := requester.scheduledRequests[id]; exists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > maxRequestThreshold {
			delete(requester.scheduledRequests, id)

			return
		}

		requester.scheduledRequests[id] = time.AfterFunc(requester.options.retryInterval, requester.createReRequest(id, count))
		return
	}
}

// RequestQueueSize returns the number of scheduled message requests.
func (requester *Requester) RequestQueueSize() int {
	requester.scheduledRequestsMutex.RLock()
	defer requester.scheduledRequestsMutex.RUnlock()
	return len(requester.scheduledRequests)
}

func (requester *Requester) createReRequest(msgID MessageID, count int) func() {
	return func() { requester.reRequest(msgID, count) }
}
