package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
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
		Events: &DispatcherEvents{
			MessageDispatched: events.NewEvent(MessageIDCaller),
		},
		tangle: tangle,
	}

	dispatcher.messageWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		messageID := task.Param(0).(MessageID)
		dispatcher.Events.MessageDispatched.Trigger(messageID)

		task.Return(nil)
	}, workerpool.WorkerCount(messageWorkerCount), workerpool.QueueSize(messageWorkerQueueSize))

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (d *Dispatcher) Setup() {
	d.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(d.onMessageScheduled))
}

// Shutdown shuts down the Dispatcher.
func (d *Dispatcher) Shutdown() {
	d.shutdownOnce.Do(func() {
		d.messageWorkerPool.Stop()
	})

	d.shutdownWG.Wait()
}

func (d *Dispatcher) onMessageScheduled(messageID MessageID) {
	d.messageWorkerPool.Submit(messageID)
}

// region DispatcherEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// DispatcherEvents represents events happening in the Dispatcher.
type DispatcherEvents struct {
	// MessageDispatched is triggered when a message is already scheduled and thus ready to be dispatched.
	MessageDispatched *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
