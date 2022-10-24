package eventticker

import (
	"time"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// region EventTicker //////////////////////////////////////////////////////////////////////////////////////////////////

// EventTicker takes care of requesting blocks.
type EventTicker[T epoch.IndexedID] struct {
	Events *Events[T]

	evictionManager      *eviction.LockableManager[T]
	timedExecutor        *timed.Executor
	scheduledTickers     *memstorage.EpochStorage[T, *timed.ScheduledTask]
	scheduledTickerCount int

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// New creates a new block requester.
func New[T epoch.IndexedID](evictionManager *eviction.State[T], opts ...options.Option[EventTicker[T]]) *EventTicker[T] {
	return options.Apply(&EventTicker[T]{
		Events: NewEvents[T](),

		evictionManager:  evictionManager.Lockable(),
		timedExecutor:    timed.NewExecutor(1),
		scheduledTickers: memstorage.NewEpochStorage[T, *timed.ScheduledTask](),

		optsRetryInterval:       10 * time.Second,
		optsRetryJitter:         5 * time.Second,
		optsMaxRequestThreshold: 100,
	}, opts, func(r *EventTicker[T]) {
		r.setup()
	})
}

func (r *EventTicker[T]) StartTicker(id T) {
	if r.addTickerToQueue(id) {
		r.Events.TickerStarted.Trigger(id)
		r.Events.Tick.Trigger(id)
	}
}

func (r *EventTicker[T]) StopTicker(id T) {
	if r.stopTicker(id) {
		r.Events.TickerStopped.Trigger(id)
	}
}

func (r *EventTicker[T]) QueueSize() int {
	r.evictionManager.RLock()
	defer r.evictionManager.RUnlock()

	return r.scheduledTickerCount
}

func (r *EventTicker[T]) Shutdown() {
	r.timedExecutor.Shutdown(timed.CancelPendingElements)
}

func (r *EventTicker[T]) setup() {
	r.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(r.evictEpoch))
}

func (r *EventTicker[T]) addTickerToQueue(id T) (added bool) {
	// TODO: RLock enough?
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	if r.evictionManager.IsTooOld(id) {
		return false
	}

	// ignore already scheduled requests
	queue := r.scheduledTickers.Get(id.Index(), true)
	if _, exists := queue.Get(id); exists {
		return false
	}

	// schedule the next request and trigger the event
	queue.Set(id, r.timedExecutor.ExecuteAfter(r.createReScheduler(id, 0), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))

	r.scheduledTickerCount++

	return true
}

func (r *EventTicker[T]) stopTicker(id T) (stopped bool) {
	// TODO: RLock enough?
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	storage := r.scheduledTickers.Get(id.Index())
	if storage == nil {
		return false
	}

	timer, exists := storage.Get(id)

	if !exists {
		return false
	}
	timer.Cancel()
	storage.Delete(id)

	r.scheduledTickerCount--

	return true
}

func (r *EventTicker[T]) reSchedule(id T, count int) {
	r.Events.Tick.Trigger(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	// reschedule, if the request has not been stopped in the meantime

	tickerStorage := r.scheduledTickers.Get(id.Index())
	if tickerStorage == nil {
		return
	}

	if _, requestExists := tickerStorage.Get(id); requestExists {
		// increase the request counter
		count++

		// if we have requested too often => stop the requests
		if count > r.optsMaxRequestThreshold {
			tickerStorage.Delete(id)

			r.scheduledTickerCount--

			r.Events.TickerFailed.Trigger(id)
			return
		}

		tickerStorage.Set(id, r.timedExecutor.ExecuteAfter(r.createReScheduler(id, count), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))
		return
	}
}

func (r *EventTicker[T]) createReScheduler(blkID T, count int) func() {
	return func() {
		r.reSchedule(blkID, count)
	}
}

func (r *EventTicker[T]) evictEpoch(epochIndex epoch.Index) {
	r.evictionManager.Lock()
	defer r.evictionManager.Unlock()

	if requestStorage := r.scheduledTickers.Get(epochIndex); requestStorage != nil {
		r.scheduledTickers.EvictEpoch(epochIndex)
		// TODO: cancel all tasks from an epoch
		r.scheduledTickerCount -= requestStorage.Size()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval[T epoch.IndexedID](interval time.Duration) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsRetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter[T epoch.IndexedID](retryJitter time.Duration) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsRetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the EventTicker should try to request blocks before
// canceling the request.
func MaxRequestThreshold[T epoch.IndexedID](maxRequestThreshold int) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsMaxRequestThreshold = maxRequestThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
