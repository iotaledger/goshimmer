package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timedexecutor"
)

const (
	// DefaultRetryInterval defines the Default Retry Interval of the message requester.
	DefaultRetryInterval = 10 * time.Second

	// the maximum amount of requests before we abort
	maxRequestThreshold = 500
)

// RequesterOptions holds options for a message requester.
type RequesterOptions struct {
	retryInterval time.Duration
}

func newRequesterOptions(optionalOptions []RequesterOption) *RequesterOptions {
	result := &RequesterOptions{
		retryInterval: DefaultRetryInterval,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

// RequesterOption is a function which inits an option.
type RequesterOption func(*RequesterOptions)

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval(interval time.Duration) RequesterOption {
	return func(args *RequesterOptions) {
		args.retryInterval = interval
	}
}

// region Requester /////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting messages.
type Requester struct {
	tangle            *Tangle
	timedExecutor     *timedexecutor.TimedExecutor
	scheduledRequests map[MessageID]*timedexecutor.ScheduledTask
	options           *RequesterOptions
	Events            *MessageRequesterEvents

	scheduledRequestsMutex sync.RWMutex
}

// NewRequester creates a new message requester.
func NewRequester(tangle *Tangle, optionalOptions ...RequesterOption) *Requester {
	requester := &Requester{
		tangle:            tangle,
		timedExecutor:     timedexecutor.New(1),
		scheduledRequests: make(map[MessageID]*timedexecutor.ScheduledTask),
		options:           newRequesterOptions(optionalOptions),
		Events: &MessageRequesterEvents{
			SendRequest:    events.NewEvent(sendRequestEventHandler),
			RequestStarted: events.NewEvent(MessageIDCaller),
			RequestStopped: events.NewEvent(MessageIDCaller),
			RequestFailed:  events.NewEvent(MessageIDCaller),
		},
	}

	// add requests for all missing messages
	requester.scheduledRequestsMutex.Lock()
	defer requester.scheduledRequestsMutex.Unlock()

	for _, id := range tangle.Storage.MissingMessages() {
		requester.scheduledRequests[id] = requester.timedExecutor.ExecuteAfter(requester.createReRequest(id, 0), requester.options.retryInterval)
	}

	return requester
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (r *Requester) Setup() {
	r.tangle.Solidifier.Events.MessageMissing.Attach(events.NewClosure(r.StartRequest))
	r.tangle.Storage.Events.MissingMessageStored.Attach(events.NewClosure(r.StopRequest))
}

// Shutdown shuts down the Requester.
func (r *Requester) Shutdown() {
	r.timedExecutor.Shutdown(timedexecutor.CancelPendingTasks)
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester) StartRequest(id MessageID) {
	r.scheduledRequestsMutex.Lock()

	// ignore already scheduled requests
	if _, exists := r.scheduledRequests[id]; exists {
		r.scheduledRequestsMutex.Unlock()
		return
	}

	// schedule the next request and trigger the event
	r.scheduledRequests[id] = r.timedExecutor.ExecuteAfter(r.createReRequest(id, 0), r.options.retryInterval)
	r.scheduledRequestsMutex.Unlock()

	r.Events.RequestStarted.Trigger(id)
}

// StopRequest stops requests for the given message to further happen.
func (r *Requester) StopRequest(id MessageID) {
	r.scheduledRequestsMutex.Lock()

	timer, ok := r.scheduledRequests[id]
	if !ok {
		r.scheduledRequestsMutex.Unlock()
		return
	}

	timer.Cancel()
	delete(r.scheduledRequests, id)
	r.scheduledRequestsMutex.Unlock()

	r.Events.RequestStopped.Trigger(id)
}

func (r *Requester) reRequest(id MessageID, count int) {
	r.Events.SendRequest.Trigger(&SendRequestEvent{ID: id})

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.scheduledRequestsMutex.Lock()
	defer r.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := r.scheduledRequests[id]; exists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > maxRequestThreshold {
			delete(r.scheduledRequests, id)

			r.Events.RequestFailed.Trigger(id)

			return
		}

		r.scheduledRequests[id] = r.timedExecutor.ExecuteAfter(r.createReRequest(id, count), r.options.retryInterval)
		return
	}
}

// RequestQueueSize returns the number of scheduled message requests.
func (r *Requester) RequestQueueSize() int {
	r.scheduledRequestsMutex.RLock()
	defer r.scheduledRequestsMutex.RUnlock()
	return len(r.scheduledRequests)
}

func (r *Requester) createReRequest(msgID MessageID, count int) func() {
	return func() { r.reRequest(msgID, count) }
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageRequesterEvents ///////////////////////////////////////////////////////////////////////////////////////

// MessageRequesterEvents represents events happening on a message requester.
type MessageRequesterEvents struct {
	// Fired when a request for a given message should be sent.
	SendRequest *events.Event

	// RequestStarted is an event that is triggered when a new request is started.
	RequestStarted *events.Event

	// RequestStopped is an event that is triggered when a request is stopped.
	RequestStopped *events.Event

	// RequestFailed is an event that is triggered when a request is stopped after too many attempts.
	RequestFailed *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SendRequestEvent /////////////////////////////////////////////////////////////////////////////////////////////

// SendRequestEvent represents the parameters of sendRequestEventHandler
type SendRequestEvent struct {
	ID MessageID
}

func sendRequestEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*SendRequestEvent))(params[0].(*SendRequestEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
