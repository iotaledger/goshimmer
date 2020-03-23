package transactionrequester

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

type TransactionRequester struct {
	scheduledRequests map[message.Id]*time.Timer
	requestWorker     async.NonBlockingWorkerPool
	options           *Options
	Events            Events

	scheduledRequestsMutex sync.RWMutex
}

func New(optionalOptions ...Option) *TransactionRequester {
	requester := &TransactionRequester{
		scheduledRequests: make(map[message.Id]*time.Timer),
		options:           newOptions(optionalOptions),
		Events: Events{
			SendRequest: events.NewEvent(func(handler interface{}, params ...interface{}) {
				handler.(func(message.Id))(params[0].(message.Id))
			}),
		},
	}

	requester.requestWorker.Tune(requester.options.workerCount)

	return requester
}

func (requester *TransactionRequester) ScheduleRequest(transactionId message.Id) {
	var retryRequest func(bool)
	retryRequest = func(initialRequest bool) {
		requester.requestWorker.Submit(func() {
			requester.scheduledRequestsMutex.RLock()
			if _, requestExists := requester.scheduledRequests[transactionId]; !initialRequest && !requestExists {
				requester.scheduledRequestsMutex.RUnlock()

				return
			}
			requester.scheduledRequestsMutex.RUnlock()

			requester.Events.SendRequest.Trigger(transactionId)

			requester.scheduledRequestsMutex.Lock()
			requester.scheduledRequests[transactionId] = time.AfterFunc(requester.options.retryInterval, func() { retryRequest(false) })
			requester.scheduledRequestsMutex.Unlock()
		})
	}

	retryRequest(true)
}

func (requester *TransactionRequester) StopRequest(transactionId message.Id) {
	requester.scheduledRequestsMutex.RLock()
	if timer, timerExists := requester.scheduledRequests[transactionId]; timerExists {
		requester.scheduledRequestsMutex.RUnlock()

		timer.Stop()

		requester.scheduledRequestsMutex.Lock()
		delete(requester.scheduledRequests, transactionId)
		requester.scheduledRequestsMutex.Unlock()
	} else {
		requester.scheduledRequestsMutex.RUnlock()
	}
}
