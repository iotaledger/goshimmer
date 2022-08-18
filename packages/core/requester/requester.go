package requester

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timedexecutor"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Requester ////////////////////////////////////////////////////////////////////////////////////////////////////

// Requester takes care of requesting blocks.
type Requester struct {
	tangle            *tangle.Tangle
	timedExecutor     *timedexecutor.TimedExecutor
	scheduledRequests *memstorage.EpochStorage[models.BlockID, *timedexecutor.ScheduledTask]
	Events            *Events

	scheduledRequestsMutex sync.RWMutex

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// NewRequester creates a new block requester.
func NewRequester(t *tangle.Tangle, opts ...options.Option[Requester]) *Requester {
	requester := &Requester{
		tangle:            t,
		timedExecutor:     timedexecutor.New(1),
		scheduledRequests: memstorage.NewEpochStorage[models.BlockID, *timedexecutor.ScheduledTask](),

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
}

// Shutdown shuts down the Requester.
func (r *Requester) Shutdown() {
	r.timedExecutor.Shutdown(timedexecutor.CancelPendingTasks)
}

// StartRequest initiates a regular triggering of the StartRequest event until it has been stopped using StopRequest.
func (r *Requester) StartRequest(id models.BlockID) {
	if r.addRequestToQueue(id) {
		return
	}

	r.Events.RequestStarted.Trigger(id)
	r.Events.RequestIssued.Trigger(id)
}

func (r *Requester) addRequestToQueue(id models.BlockID) (added bool) {
	r.scheduledRequestsMutex.Lock()
	defer r.scheduledRequestsMutex.Unlock()

	// TODO: check if too old

	// ignore already scheduled requests
	if _, exists := r.scheduledRequests.Get(id.Index(), true).Get(id); exists {
		return true
	}

	// schedule the next request and trigger the event
	r.scheduledRequests.Get(id.Index()).Set(id, r.timedExecutor.ExecuteAfter(r.createReRequest(id, 0), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))
	return false
}

// StopRequest stops requests for the given block to further happen.
func (r *Requester) StopRequest(id models.BlockID) {
	r.scheduledRequestsMutex.Lock()

	storage := r.scheduledRequests.Get(id.Index())
	if storage == nil {
		r.scheduledRequestsMutex.Unlock()
		return
	}

	timer, exists := storage.Get(id)

	if !exists {
		r.scheduledRequestsMutex.Unlock()
		return
	}

	timer.Cancel()
	storage.Delete(id)
	r.scheduledRequestsMutex.Unlock()

	r.Events.RequestStopped.Trigger(id)
}

func (r *Requester) reRequest(id models.BlockID, count int) {
	r.Events.RequestIssued.Trigger(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.scheduledRequestsMutex.Lock()
	defer r.scheduledRequestsMutex.Unlock()

	// reschedule, if the request has not been stopped in the meantime
	if _, exists := r.scheduledRequests[id]; exists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > r.optsMaxRequestThreshold {
			delete(r.scheduledRequests, id)

			r.Events.RequestFailed.Trigger(id)
			return
		}

		r.scheduledRequests[id] = r.timedExecutor.ExecuteAfter(r.createReRequest(id, count), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter)))
		return
	}
}

// RequestQueueSize returns the number of scheduled block requests.
func (r *Requester) RequestQueueSize() int {
	r.scheduledRequestsMutex.RLock()
	defer r.scheduledRequestsMutex.RUnlock()
	return len(r.scheduledRequests)
}

func (r *Requester) createReRequest(blkID models.BlockID, count int) func() {
	return func() { r.reRequest(blkID, count) }
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
