package activitytracker

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// TimeRetrieverFunc is a function type to retrieve the time.
type TimeRetrieverFunc func() time.Time

// region ActivityTracker //////////////////////////////////////////////////////////////////////////////////////////

// ActivityTracker is a component that keeps track of active nodes based on their time-based activity in relation to activeTimeThreshold.
type ActivityTracker struct {
	timedExecutor     *TimedTaskExecutor
	lastActiveMap     *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	timeRetrieverFunc TimeRetrieverFunc
	validatorSet      *validator.Set

	optsWorkersCount   uint
	optsActivityWindow time.Duration

	mutex sync.RWMutex
}

func New(validatorSet *validator.Set, timeRetrieverFunc TimeRetrieverFunc, opts ...options.Option[ActivityTracker]) (activityTracker *ActivityTracker) {
	return options.Apply(&ActivityTracker{
		timeRetrieverFunc: timeRetrieverFunc,
		validatorSet:      validatorSet,
		lastActiveMap:     shrinkingmap.New[identity.ID, time.Time](),

		optsWorkersCount:   1,
		optsActivityWindow: time.Second * 30,
	}, opts, func(a *ActivityTracker) {
		a.timedExecutor = NewTimedTaskExecutor(int(a.optsWorkersCount))
	})
}

// Update updates the underlying data structure and keeps track of active nodes.
func (a *ActivityTracker) Update(validator *validator.Validator, activityTime time.Time) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	issuerLastActivity, exists := a.lastActiveMap.Get(validator.ID())
	if exists && issuerLastActivity.After(activityTime) {
		return
	}

	a.lastActiveMap.Set(validator.ID(), activityTime)
	a.validatorSet.Add(validator)

	a.timedExecutor.ExecuteAfter(validator.ID(), func() {
		a.mutex.Lock()
		defer a.mutex.Unlock()

		a.lastActiveMap.Delete(validator.ID())
		a.validatorSet.Delete(validator)
	}, activityTime.Add(a.optsActivityWindow).Sub(a.timeRetrieverFunc()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithWorkersCount(workersCount uint) options.Option[ActivityTracker] {
	return func(a *ActivityTracker) {
		a.optsWorkersCount = workersCount
	}
}

func WithActivityWindow(activityWindow time.Duration) options.Option[ActivityTracker] {
	return func(a *ActivityTracker) {
		a.optsActivityWindow = activityWindow
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
