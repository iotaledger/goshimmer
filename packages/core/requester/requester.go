package requester

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/timedexecutor"

	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting blocks.
type Requester struct {
	tangle            *tangle.Tangle
	timedExecutor     *timedexecutor.TimedExecutor
	scheduledRequests map[tangle.BlockID]*timedexecutor.ScheduledTask
	options           RequesterOptions
	Events            *Events

	scheduledRequestsMutex sync.RWMutex
}

// NewRequester creates a new block requester.
func NewRequester(t *tangle.Tangle, optionalOptions ...RequesterOption) *Requester {
	requester := &Requester{
		tangle:            t,
		timedExecutor:     timedexecutor.New(1),
		scheduledRequests: make(map[tangle.BlockID]*timedexecutor.ScheduledTask),
		options:           DefaultRequesterOptions.Apply(optionalOptions...),
		Events:            newEvents(),
	}

	return requester
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (r *Requester) Setup() {
	r.tangle.Events.BlockMissing.Hook(event.NewClosure(func(metadata *tangle.BlockMetadata) {
		r.StartRequest(metadata.ID())
	}))
	r.tangle.Events.MissingBlockStored.Hook(event.NewClosure(func(metadata *tangle.BlockMetadata) {
		r.StopRequest(metadata.ID())
	}))
}

// Shutdown shuts down the Requester.
func (r *Requester) Shutdown() {
	r.timedExecutor.Shutdown(timedexecutor.CancelPendingTasks)
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester) StartRequest(id tangle.BlockID) {
	r.scheduledRequestsMutex.Lock()

	// ignore already scheduled requests
	if _, exists := r.scheduledRequests[id]; exists {
		r.scheduledRequestsMutex.Unlock()
		return
	}

	// schedule the next request and trigger the event
	r.scheduledRequests[id] = r.timedExecutor.ExecuteAfter(r.createReRequest(id, 0), r.options.RetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.options.RetryJitter)))
	r.scheduledRequestsMutex.Unlock()

	r.Events.RequestStarted.Trigger(id)
	r.Events.RequestIssued.Trigger(id)
}

// StopRequest stops requests for the given block to further happen.
func (r *Requester) StopRequest(id tangle.BlockID) {
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

func (r *Requester) reRequest(id tangle.BlockID, count int) {
	r.Events.RequestIssued.Trigger(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.scheduledRequestsMutex.Lock()
	defer r.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := r.scheduledRequests[id]; exists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > r.options.MaxRequestThreshold {
			delete(r.scheduledRequests, id)

			r.Events.RequestFailed.Trigger(id)
			r.tangle.Storage.DeleteMissingBlock(id)

			return
		}

		r.scheduledRequests[id] = r.timedExecutor.ExecuteAfter(r.createReRequest(id, count), r.options.RetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.options.RetryJitter)))
		return
	}
}

// RequestQueueSize returns the number of scheduled block requests.
func (r *Requester) RequestQueueSize() int {
	r.scheduledRequestsMutex.RLock()
	defer r.scheduledRequestsMutex.RUnlock()
	return len(r.scheduledRequests)
}

func (r *Requester) createReRequest(blkID tangle.BlockID, count int) func() {
	return func() { r.reRequest(blkID, count) }
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RequesterOptions /////////////////////////////////////////////////////////////////////////////////////////////

// DefaultRequesterOptions defines the default options that are used when creating Requester instances.
var DefaultRequesterOptions = &Options{
	RetryInterval:       10 * time.Second,
	RetryJitter:         10 * time.Second,
	MaxRequestThreshold: 500,
}

// Options holds options for a block requester.
type Options struct {
	// RetryInterval represents an option which defines in which intervals the Requester will try to ask for missing
	// blocks.
	RetryInterval time.Duration

	// RetryJitter defines how much the RetryInterval should be randomized, so that the nodes don't always send blocks
	// at exactly the same interval.
	RetryJitter time.Duration

	// MaxRequestThreshold represents an option which defines how often the Requester should try to request blocks
	// before canceling the request
	MaxRequestThreshold int
}

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval(interval time.Duration) options.Option[Requester] {
	return func(requester *Requester) {
		args.RetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter(retryJitter time.Duration) RequesterOption {
	return func(args *Options) {
		args.RetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the Requester should try to request blocks before
// canceling the request.
func MaxRequestThreshold(maxRequestThreshold int) RequesterOption {
	return func(args *Options) {
		args.MaxRequestThreshold = maxRequestThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
