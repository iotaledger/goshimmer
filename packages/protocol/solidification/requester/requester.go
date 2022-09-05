package requester

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timedexecutor"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting blocks.
type Requester struct {
	Events *Events

	evictionManager        *eviction.LockableManager[models.BlockID]
	timedExecutor          *timedexecutor.TimedExecutor
	scheduledRequests      *memstorage.EpochStorage[models.BlockID, *timedexecutor.ScheduledTask]
	scheduledRequestsCount int

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// New creates a new block requester.
func New(evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[Requester]) *Requester {
	return options.Apply(&Requester{
		Events: NewEvents(),

		evictionManager:   evictionManager.Lockable(),
		timedExecutor:     timedexecutor.New(1),
		scheduledRequests: memstorage.NewEpochStorage[models.BlockID, *timedexecutor.ScheduledTask](),

		optsRetryInterval:       10 * time.Second,
		optsRetryJitter:         10 * time.Second,
		optsMaxRequestThreshold: 100,
	}, opts, (*Requester).setup)
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester) StartRequest(block *blockdag.Block) {
	if id := block.ID(); r.addRequestToQueue(id) {
		r.Events.RequestStarted.Trigger(id)
		r.Events.BlockRequested.Trigger(id)
	}
}

// StopRequest stops requests for the given block to further happen.
func (r *Requester) StopRequest(block *blockdag.Block) {
	if id := block.ID(); r.stopRequest(id) {
		r.Events.RequestStopped.Trigger(id)
	}
}

// QueueSize returns the number of scheduled block requests.
func (r *Requester) QueueSize() int {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	return r.scheduledRequestsCount
}

// Shutdown shuts down the Requester.
func (r *Requester) Shutdown() {
	r.timedExecutor.Shutdown(timedexecutor.CancelPendingTasks)
}

// setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (r *Requester) setup() {
	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.evictEpoch))
}

func (r *Requester) addRequestToQueue(id models.BlockID) (added bool) {
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

func (r *Requester) stopRequest(id models.BlockID) (stopped bool) {
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

func (r *Requester) reRequest(id models.BlockID, count int) {
	r.Events.BlockRequested.Trigger(id)

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

func (r *Requester) createReRequest(blkID models.BlockID, count int) func() {
	return func() { r.reRequest(blkID, count) }
}

func (r *Requester) evictEpoch(epochIndex epoch.Index) {
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
func RetryInterval(interval time.Duration) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsRetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter(retryJitter time.Duration) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsRetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the Requester should try to request blocks before
// canceling the request.
func MaxRequestThreshold(maxRequestThreshold int) options.Option[Requester] {
	return func(requester *Requester) {
		requester.optsMaxRequestThreshold = maxRequestThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
