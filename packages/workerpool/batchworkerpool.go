package workerpool

import (
	"runtime"
	"time"
)

type BatchWorkerPoolOptions struct {
	WorkerCount            int
	QueueSize              int
	MaxBatchSize           int
	BatchCollectionTimeout time.Duration
}

type BatchWorkerPool struct {
	workerFnc        func([]Call)
	options          BatchWorkerPoolOptions
	callsChan        chan Call
	batchedCallsChan chan []Call
}

func NewBatchWorkerPool(workerFnc func([]Call), options BatchWorkerPoolOptions) *BatchWorkerPool {
	return &BatchWorkerPool{
		workerFnc: workerFnc,
		options:   options,

		callsChan:        make(chan Call, options.QueueSize),
		batchedCallsChan: make(chan []Call, 2*options.WorkerCount),
	}
}

func (wp *BatchWorkerPool) Submit(params ...interface{}) (result chan interface{}) {
	result = make(chan interface{}, 1)

	wp.callsChan <- Call{
		params:     params,
		resultChan: result,
	}

	return
}

func (wp *BatchWorkerPool) Start() {
	wp.startBatchDispatcher()
	wp.startBatchWorkers()
}

func (wp *BatchWorkerPool) startBatchDispatcher() {
	go func() {
		for {
			// wait for first request to start processing at all
			batchTask := append(make([]Call, 0), <-wp.callsChan)

			collectionTimeout := time.After(wp.options.BatchCollectionTimeout)

			// collect additional requests that arrive within the timeout
		CollectAdditionalCalls:
			for {
				select {
				case <-collectionTimeout:
					break CollectAdditionalCalls
				case call := <-wp.callsChan:
					batchTask = append(batchTask, call)

					if len(batchTask) == wp.options.MaxBatchSize {
						break CollectAdditionalCalls
					}
				}
			}

			wp.batchedCallsChan <- batchTask
		}
	}()
}

func (wp *BatchWorkerPool) startBatchWorkers() {
	for i := 0; i < wp.options.WorkerCount; i++ {
		go func() {
			for {
				batchTask := <-wp.batchedCallsChan

				wp.workerFnc(batchTask)
			}
		}()
	}
}

var (
	DEFAULT_OPTIONS = BatchWorkerPoolOptions{
		WorkerCount:            2 * runtime.NumCPU(),
		QueueSize:              500,
		MaxBatchSize:           64,
		BatchCollectionTimeout: 10 * time.Millisecond,
	}
)
