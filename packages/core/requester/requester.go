package requester

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timedexecutor"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting blocks.
type Requester struct {
	tangle                 *tangle.Tangle
	timedExecutor          *timedexecutor.TimedExecutor
	scheduledRequests      *memstorage.EpochStorage[models.BlockID, *timedexecutor.ScheduledTask]
	scheduledRequestsCount int
	evictionManager        *eviction.LockableManager
	Events                 *Events

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// NewRequester creates a new block requester.
func NewRequester(t *tangle.Tangle, evictionManager *eviction.LockableManager, opts ...options.Option[Requester]) *Requester {
	requester := &Requester{
		tangle:            t,
		timedExecutor:     timedexecutor.New(1),
		scheduledRequests: memstorage.NewEpochStorage[models.BlockID, *timedexecutor.ScheduledTask](),
		evictionManager:   evictionManager.Lockable(),

		optsRetryInterval:       10 * time.Second,
		optsRetryJitter:         10 * time.Second,
		optsMaxRequestThreshold: 500,

		Events: newEvents(),
	}

	options.Apply(requester, opts)

	return requester
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (r *Requester) Setup() {
	r.tangle.Events.BlockMissing.Hook(event.NewClosure(func(block *tangle.Block) {
		r.StartRequest(block.ID())
	}))
	r.tangle.Events.MissingBlockAttached.Hook(event.NewClosure(func(block *tangle.Block) {
		r.StopRequest(block.ID())
	}))

	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.evictEpoch))

}

// Shutdown shuts down the Requester.
func (r *Requester) Shutdown() {
	r.timedExecutor.Shutdown(timedexecutor.CancelPendingTasks)
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester) StartRequest(id models.BlockID) {
	if !r.addRequestToQueue(id) {
		return
	}

	r.Events.RequestStarted.Trigger(id)
	r.Events.RequestIssued.Trigger(id)
}

func (r *Requester) addRequestToQueue(id models.BlockID) (added bool) {
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	if r.evictionManager.IsTooOld(id) {
		return false
	}

	// ignore already scheduled requests
	if _, exists := r.scheduledRequests.Get(id.Index(), true).Get(id); exists {
		return false
	}

	// schedule the next request and trigger the event
	r.scheduledRequests.Get(id.Index()).Set(id, r.timedExecutor.ExecuteAfter(r.createReRequest(id, 0), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))

	r.scheduledRequestsCount++

	return true
}

// StopRequest stops requests for the given block to further happen.
func (r *Requester) StopRequest(id models.BlockID) {
	if !r.stopRequest(id) {
		return
	}

	r.Events.RequestStopped.Trigger(id)
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
	r.Events.RequestIssued.Trigger(id)

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

// RequestQueueSize returns the number of scheduled block requests.
func (r *Requester) RequestQueueSize() int {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	return r.scheduledRequestsCount
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
