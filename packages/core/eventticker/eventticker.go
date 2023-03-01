package eventticker

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/timed"
)

// region EventTicker //////////////////////////////////////////////////////////////////////////////////////////////////

// EventTicker takes care of requesting blocks.
type EventTicker[T slot.IndexedID] struct {
	Events *Events[T]

	timedExecutor             *timed.Executor
	scheduledTickers          *memstorage.SlotStorage[T, *timed.ScheduledTask]
	scheduledTickerCount      int
	scheduledTickerCountMutex sync.RWMutex
	lastEvictedSlot           slot.Index
	evictionMutex             sync.RWMutex

	optsRetryInterval       time.Duration
	optsRetryJitter         time.Duration
	optsMaxRequestThreshold int
}

// New creates a new block requester.
func New[T slot.IndexedID](opts ...options.Option[EventTicker[T]]) *EventTicker[T] {
	return options.Apply(&EventTicker[T]{
		Events: NewEvents[T](),

		timedExecutor:    timed.NewExecutor(1),
		scheduledTickers: memstorage.NewSlotStorage[T, *timed.ScheduledTask](),

		optsRetryInterval:       10 * time.Second,
		optsRetryJitter:         5 * time.Second,
		optsMaxRequestThreshold: 100,
	}, opts)
}

func (r *EventTicker[T]) StartTickers(ids []T) {
	for _, id := range ids {
		r.StartTicker(id)
	}
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

func (r *EventTicker[T]) HasTicker(id T) bool {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	if id.Index() <= r.lastEvictedSlot {
		return false
	}

	if queue := r.scheduledTickers.Get(id.Index(), false); queue != nil {
		return queue.Has(id)
	}

	return false
}

func (r *EventTicker[T]) QueueSize() int {
	r.scheduledTickerCountMutex.RLock()
	defer r.scheduledTickerCountMutex.RUnlock()

	return r.scheduledTickerCount
}

func (r *EventTicker[T]) EvictUntil(index slot.Index) {
	r.evictionMutex.Lock()
	defer r.evictionMutex.Unlock()

	if index <= r.lastEvictedSlot {
		return
	}

	for currentIndex := r.lastEvictedSlot + 1; currentIndex <= index; currentIndex++ {
		if evictedStorage := r.scheduledTickers.Evict(currentIndex); evictedStorage != nil {
			evictedStorage.ForEach(func(id T, scheduledTask *timed.ScheduledTask) bool {
				scheduledTask.Cancel()

				return true
			})

			r.updateScheduledTickerCount(-evictedStorage.Size())
		}
	}
	r.lastEvictedSlot = index
}

func (r *EventTicker[T]) Shutdown() {
	r.timedExecutor.Shutdown(timed.CancelPendingElements)
}

func (r *EventTicker[T]) addTickerToQueue(id T) (added bool) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

	if id.Index() <= r.lastEvictedSlot {
		return false
	}

	// ignore already scheduled requests
	queue := r.scheduledTickers.Get(id.Index(), true)
	if _, exists := queue.Get(id); exists {
		return false
	}

	// schedule the next request and trigger the event
	queue.Set(id, r.timedExecutor.ExecuteAfter(r.createReScheduler(id, 0), r.optsRetryInterval+time.Duration(crypto.Randomness.Float64()*float64(r.optsRetryJitter))))

	r.updateScheduledTickerCount(1)

	return true
}

func (r *EventTicker[T]) stopTicker(id T) (stopped bool) {
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

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

	r.updateScheduledTickerCount(-1)

	return true
}

func (r *EventTicker[T]) reSchedule(id T, count int) {
	r.Events.Tick.Trigger(id)

	// as we schedule a request at most once per id we do not need to make the trigger and the re-schedule atomic
	r.evictionMutex.RLock()
	defer r.evictionMutex.RUnlock()

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

			r.updateScheduledTickerCount(-1)

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

func (r *EventTicker[T]) updateScheduledTickerCount(diff int) {
	r.scheduledTickerCountMutex.Lock()
	defer r.scheduledTickerCountMutex.Unlock()

	r.scheduledTickerCount += diff
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// RetryInterval creates an option which sets the retry interval to the given value.
func RetryInterval[T slot.IndexedID](interval time.Duration) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsRetryInterval = interval
	}
}

// RetryJitter creates an option which sets the retry jitter to the given value.
func RetryJitter[T slot.IndexedID](retryJitter time.Duration) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsRetryJitter = retryJitter
	}
}

// MaxRequestThreshold creates an option which defines how often the EventTicker should try to request blocks before
// canceling the request.
func MaxRequestThreshold[T slot.IndexedID](maxRequestThreshold int) options.Option[EventTicker[T]] {
	return func(requester *EventTicker[T]) {
		requester.optsMaxRequestThreshold = maxRequestThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
