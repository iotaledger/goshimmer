package requester

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting blocks.
type Requester[T epoch.IndexedID] struct {
	Events *Events[T]

	evictionManager        *eviction.LockableManager[T]
	timedExecutor          *timed.Executor
	scheduledRequests      *memstorage.EpochStorage[T, *timed.ScheduledTask]
	scheduledRequestsCount int

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// New creates a new block requester.
func NewRequester[T epoch.IndexedID](evictionManager *eviction.Manager[T], opts ...options.Option[Requester[T]]) *Requester[T] {
	return options.Apply(&Requester[T]{
		Events: NewEvents[T](),

		evictionManager:   evictionManager.Lockable(),
		timedExecutor:     timed.NewExecutor(1),
		scheduledRequests: memstorage.NewEpochStorage[T, *timed.ScheduledTask](),

		optsRetryInterval:       10 * time.Second,
		optsRetryJitter:         10 * time.Second,
		optsMaxRequestThreshold: 100,
	}, opts, func(r *Requester[T]) {
		r.setup()
	})
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester[T]) StartRequest(id T) {
	if r.addRequestToQueue(id) {
		r.Events.RequestQueued.Trigger(id)
		r.Events.Request.Trigger(id)
	}
}

// StopRequest stops requests for the given block to further happen.
func (r *Requester[T]) StopRequest(id T) {
	if r.stopRequest(id) {
		r.Events.RequestStopped.Trigger(id)
	}
}

// QueueSize returns the number of scheduled block requests.
func (r *Requester[T]) QueueSize() int {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	return r.scheduledRequestsCount
}

// Shutdown shuts down the Requester.
func (r *Requester[T]) Shutdown() {
	r.timedExecutor.Shutdown(timed.CancelPendingElements)
}

// setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (r *Requester[T]) setup() {
	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.evictEpoch))
}

func (r *Requester[T]) addRequestToQueue(id T) (added bool) {
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	if r.evictionManager.IsTooOld(id) {
		return false
	}

	// ignore already scheduled requests
	queue := r.scheduledRequests.Get(id.Index(), true)
	if _, exists := queue.Get(id); exists {
		return false
	}

	// schedule the next request and trigger the event
	queue.Set(id, r.timedExecutor.ExecuteAfter(r.createReRequest(id, 0), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))

	r.scheduledRequestsCount++

	return true
}

func (r *Requester[T]) stopRequest(id T) (stopped bool) {
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	storage := r.scheduledRequests.Get(id.Index())
	if storage == nil {
		return false
	}

	timer, exists := storage.Get(id)

	if !exists {
		return false
	}

	timer.Cancel()
	storage.Delete(id)

	r.scheduledRequestsCount--

	return true
}

func (r *Requester[T]) reRequest(id T, count int) {
	r.Events.Request.Trigger(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	// reschedule, if the request has not been stopped in the meantime

	requestStorage := r.scheduledRequests.Get(id.Index())
	if requestStorage == nil {
		return
	}

	if _, requestExists := requestStorage.Get(id); requestExists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > r.optsMaxRequestThreshold {
			requestStorage.Delete(id)

			r.scheduledRequestsCount--

			r.Events.RequestFailed.Trigger(id)
			return
		}

		requestStorage.Set(id, r.timedExecutor.ExecuteAfter(r.createReRequest(id, count), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))
		return
	}
}

func (r *Requester[T]) createReRequest(blkID T, count int) func() {
	return func() { r.reRequest(blkID, count) }
}

func (r *Requester[T]) evictEpoch(epochIndex epoch.Index) {
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	if requestStorage := r.scheduledRequests.Get(epochIndex); requestStorage != nil {
		r.scheduledRequests.EvictEpoch(epochIndex)
		r.scheduledRequestsCount -= requestStorage.Size()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval[T epoch.IndexedID](interval time.Duration) options.Option[Requester[T]] {
	return func(requester *Requester[T]) {
		requester.optsRetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter[T epoch.IndexedID](retryJitter time.Duration) options.Option[Requester[T]] {
	return func(requester *Requester[T]) {
		requester.optsRetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the Requester should try to request blocks before
// canceling the request.
func MaxRequestThreshold[T epoch.IndexedID](maxRequestThreshold int) options.Option[Requester[T]] {
	return func(requester *Requester[T]) {
		requester.optsMaxRequestThreshold = maxRequestThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
