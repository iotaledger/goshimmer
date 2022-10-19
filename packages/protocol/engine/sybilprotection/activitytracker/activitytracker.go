package activitytracker

import (
	"fmt"
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

// ActivityTracker is a component that keeps track of active nodes based on their time-based activity in relation to activeTimeThreshold.
type ActivityTracker struct {
	timedExecutor     *timed.TaskExecutor[identity.ID]
	lastActiveMap     *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	timeRetrieverFunc TimeRetrieverFunc
	validatorSet      *validator.Set

	optsWorkersCount   uint
	optsActivityWindow time.Duration

	mutex sync.RWMutex
}

// New creates and returns a new instance of an ActivityTracker.
func New(validatorSet *validator.Set, timeRetrieverFunc TimeRetrieverFunc, opts ...options.Option[ActivityTracker]) (activityTracker *ActivityTracker) {
	return options.Apply(&ActivityTracker{
		timeRetrieverFunc: timeRetrieverFunc,
		validatorSet:      validatorSet,
		lastActiveMap:     shrinkingmap.New[identity.ID, time.Time](),

		optsWorkersCount:   1,
		optsActivityWindow: time.Second * 30,
	}, opts, func(a *ActivityTracker) {
		a.timedExecutor = timed.NewTaskExecutor[identity.ID](int(a.optsWorkersCount))
	})
}

// Update updates the underlying data structure and keeps track of active nodes.
func (a *ActivityTracker) Update(activeValidator *validator.Validator, activityTime time.Time) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	fmt.Printf("Update activity %s: %s\n", activeValidator.ID(), activityTime)

	issuerLastActivity, exists := a.lastActiveMap.Get(activeValidator.ID())
	if exists && issuerLastActivity.After(activityTime) {
		return
	}

	fmt.Printf("Update activity %s: %s - last activity: %s - starting task in: %s\n", activeValidator.ID(), activityTime, issuerLastActivity, activityTime.Add(a.optsActivityWindow).Sub(a.timeRetrieverFunc()))

	a.lastActiveMap.Set(activeValidator.ID(), activityTime)
	a.validatorSet.Add(activeValidator)

	a.timedExecutor.ExecuteAfter(activeValidator.ID(), func() {
		a.mutex.Lock()
		defer a.mutex.Unlock()

		a.lastActiveMap.Delete(activeValidator.ID())
		a.validatorSet.Delete(activeValidator)
	}, activityTime.Add(a.optsActivityWindow).Sub(a.timeRetrieverFunc()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithWorkersCount sets the amount of background workers of the task executor of ActivityTracker.
func WithWorkersCount(workersCount uint) options.Option[ActivityTracker] {
	return func(a *ActivityTracker) {
		a.optsWorkersCount = workersCount
	}
}

// WithActivityWindow sets the duration for which a validator is recognized as active after issuing a block.
func WithActivityWindow(activityWindow time.Duration) options.Option[ActivityTracker] {
	return func(a *ActivityTracker) {
		a.optsActivityWindow = activityWindow
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
