package traits

import (
	"fmt"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/workerpool"
)

type Name = string

// Runnable is a trait that allows to run and get the workerpools.
type Runnable interface {
	Run()

	Shutdown()

	WorkerPools() map[Name]*workerpool.UnboundedWorkerPool

	NewWorkerPool(name Name, optWorkerCount ...int) *workerpool.UnboundedWorkerPool

	WaitWorkerPoolsEmpty()

	AttachRunnable(name Name, other Runnable)
	DetachRunnable(name Name)

	Stoppable
}

func NewRunnable(optStoppable ...Stoppable) Runnable {
	var s Stoppable
	if len(optStoppable) > 0 {
		s = optStoppable[0]
	} else {
		s = NewStoppable()
	}

	return &runnable{
		workerPools:       make(map[Name]*workerpool.UnboundedWorkerPool),
		attachedRunnables: make(map[Name]*attachedRunnable),
		Stoppable:         s,
	}
}

type attachedRunnable struct {
	detachShutdown func()
	Runnable
}

type runnable struct {
	workerPools       map[Name]*workerpool.UnboundedWorkerPool
	attachedRunnables map[Name]*attachedRunnable
	running           bool
	Stoppable
}

func (r *runnable) NewWorkerPool(name Name, optWorkerCount ...int) *workerpool.UnboundedWorkerPool {
	wp, has := r.workerPools[name]
	if !has {
		wp = workerpool.NewUnboundedWorkerPool(optWorkerCount...)
		r.workerPools[name] = wp
	}
	if r.running {
		wp.Start()
	}
	return wp
}

func (r *runnable) Run() {
	r.running = true
	for _, run := range r.attachedRunnables {
		run.Run()
	}
	for _, wp := range r.workerPools {
		wp.Start()
	}
}

func (r *runnable) Shutdown() {
	r.TriggerStopped()
	r.WaitWorkerPoolsEmpty()
	for _, wp := range r.workerPools {
		wp.Shutdown().ShutdownComplete.Wait()
	}
	r.running = false
}

func (r *runnable) WorkerPools() map[Name]*workerpool.UnboundedWorkerPool {
	wp := make(map[Name]*workerpool.UnboundedWorkerPool)
	lo.MergeMaps(wp, r.workerPools)
	for prefix, run := range r.attachedRunnables {
		for name, pool := range run.WorkerPools() {
			wp[fmt.Sprintf("%s.%s", prefix, name)] = pool
		}
	}
	return wp
}

func (r *runnable) WaitWorkerPoolsEmpty() {
	for _, pool := range r.WorkerPools() {
		pool.PendingTasksCounter.WaitIsZero()
	}
}

func (r *runnable) AttachRunnable(name Name, other Runnable) {
	if _, has := r.attachedRunnables[name]; has {
		panic(errors.Errorf("runnable with name %s already attached", name))
	}

	unsubscribe := r.SubscribeStopped(other.Shutdown)

	r.attachedRunnables[name] = &attachedRunnable{
		detachShutdown: unsubscribe,
		Runnable:       other,
	}
}

func (r *runnable) DetachRunnable(name Name) {
	if attached, has := r.attachedRunnables[name]; has {
		attached.detachShutdown()
	}

	delete(r.attachedRunnables, name)
}
