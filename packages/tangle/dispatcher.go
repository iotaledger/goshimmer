package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/workerpool"
)

const (
	messageWorkerCount     = 1
	messageWorkerQueueSize = 1024
)

// Dispatcher is a Tangle component that dispatches messages after being scheduled.
type Dispatcher struct {
	Events *DispatcherEvents

	tangle       *Tangle
	shutdownWG   sync.WaitGroup
	shutdownOnce sync.Once

	messageWorkerPool *workerpool.NonBlockingQueuedWorkerPool
}

// NewDispatcher is the constructor for the Dispatcher.
func NewDispatcher(tangle *Tangle) (dispatcher *Dispatcher) {
	dispatcher = &Dispatcher{
		Events: newDispatcherEvents(),
		tangle: tangle,
	}

	dispatcher.messageWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		messageID := task.Param(0).(MessageID)
		dispatcher.Events.MessageDispatched.Trigger(&MessageDispatchedEvent{messageID})

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (d *Dispatcher) Setup() {
	d.tangle.Scheduler.Events.MessageScheduled.Attach(event.NewClosure(func(event *MessageScheduledEvent) {
		d.messageWorkerPool.Submit(event.MessageID)
	}))
}

// Shutdown shuts down the Dispatcher.
func (d *Dispatcher) Shutdown() {
	d.shutdownOnce.Do(func() {
		d.messageWorkerPool.Stop()
	})

	d.shutdownWG.Wait()
}
