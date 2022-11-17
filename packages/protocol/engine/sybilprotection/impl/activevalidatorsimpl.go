package impl

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// TimeRetrieverFunc is a function type to retrieve the time.
type TimeRetrieverFunc func() time.Time

// region ActivityTracker //////////////////////////////////////////////////////////////////////////////////////////

// ActiveValidators is a component that keeps track of active nodes based on their time-based activity in relation to activeTimeThreshold.
type ActiveValidators struct {
	timedExecutor     *timed.TaskExecutor[identity.ID]
	lastActiveMap     *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	timeRetrieverFunc TimeRetrieverFunc
	validatorSet      *validator.Set

	optsWorkersCount   uint
	optsActivityWindow time.Duration

	mutex sync.RWMutex
}

// New creates and returns a new instance of an ActivityTracker.
func New(timeRetrieverFunc TimeRetrieverFunc, opts ...options.Option[ActiveValidators]) (activityTracker *ActiveValidators) {
	return options.Apply(&ActiveValidators{
		timeRetrieverFunc: timeRetrieverFunc,
		validatorSet:      validator.NewSet(),
		lastActiveMap:     shrinkingmap.New[identity.ID, time.Time](),

		optsWorkersCount:   1,
		optsActivityWindow: time.Second * 30,
	}, opts, func(a *ActiveValidators) {
		a.timedExecutor = timed.NewTaskExecutor[identity.ID](int(a.optsWorkersCount))
	})
}

// Set updates the underlying data structure and keeps track of active nodes.
func (a *ActiveValidators) Set(activeValidator *validator.Validator, activityTime time.Time) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	issuerLastActivity, exists := a.lastActiveMap.Get(activeValidator.ID())
	if exists && issuerLastActivity.After(activityTime) {
		return
	}

	if !exists {
		a.validatorSet.Add(activeValidator)
	}

	a.lastActiveMap.Set(activeValidator.ID(), activityTime)

	a.timedExecutor.ExecuteAfter(activeValidator.ID(), func() {
		a.mutex.Lock()
		defer a.mutex.Unlock()

		a.lastActiveMap.Delete(activeValidator.ID())
		a.validatorSet.Delete(activeValidator)
	}, activityTime.Add(a.optsActivityWindow).Sub(a.timeRetrieverFunc()))
}

func (a *ActiveValidators) Get(id identity.ID) (validator *validator.Validator, exists bool) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.validatorSet.Get(id)
}

func (a *ActiveValidators) ForEach(callback func(id identity.ID, validator *validator.Validator) bool) {
	a.validatorSet.ForEach(callback)
}

func (a *ActiveValidators) Weight() int64 {
	return a.validatorSet.TotalWeight()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithWorkersCount sets the amount of background workers of the task executor of ActivityTracker.
func WithWorkersCount(workersCount uint) options.Option[ActiveValidators] {
	return func(a *ActiveValidators) {
		a.optsWorkersCount = workersCount
	}
}

// WithActivityWindow sets the duration for which a validator is recognized as active after issuing a block.
func WithActivityWindow(activityWindow time.Duration) options.Option[ActiveValidators] {
	return func(a *ActiveValidators) {
		a.optsActivityWindow = activityWindow
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
