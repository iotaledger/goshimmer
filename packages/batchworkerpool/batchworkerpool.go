package batchworkerpool

import (
	"sync"
	"time"
)

type BatchWorkerPool struct {
	workerFnc func([]Task)
	options   *Options

	calls        chan Task
	batchedCalls chan []Task
	terminate    chan int

	running bool
	mutex   sync.RWMutex
	wait    sync.WaitGroup
}

func New(workerFnc func([]Task), optionalOptions ...Option) (result *BatchWorkerPool) {
	options := DEFAULT_OPTIONS.Override(optionalOptions...)

	result = &BatchWorkerPool{
		workerFnc: workerFnc,
		options:   options,
	}

	result.resetChannels()

	return
}

func (wp *BatchWorkerPool) Submit(params ...interface{}) (result chan interface{}) {
	result = make(chan interface{}, 1)

	wp.mutex.RLock()

	if wp.running {
		wp.calls <- Task{
			params:     params,
			resultChan: result,
		}
	} else {
		close(result)
	}

	wp.mutex.RUnlock()

	return
}

func (wp *BatchWorkerPool) Start() {
	wp.mutex.Lock()

	if !wp.running {
		wp.running = true

		wp.startBatchDispatcher()
		wp.startBatchWorkers()
	}

	wp.mutex.Unlock()
}

func (wp *BatchWorkerPool) Run() {
	wp.Start()

	wp.wait.Wait()
}

func (wp *BatchWorkerPool) Stop() {
	go wp.StopAndWait()
}

func (wp *BatchWorkerPool) StopAndWait() {
	wp.mutex.Lock()

	if wp.running {
		wp.running = false

		close(wp.terminate)
		wp.resetChannels()
	}

	wp.wait.Wait()

	wp.mutex.Unlock()
}

func (wp *BatchWorkerPool) resetChannels() {
	wp.calls = make(chan Task, wp.options.QueueSize)
	wp.batchedCalls = make(chan []Task, 2*wp.options.WorkerCount)
	wp.terminate = make(chan int, 1)
}

func (wp *BatchWorkerPool) startBatchDispatcher() {
	calls := wp.calls
	terminate := wp.terminate

	wp.wait.Add(1)

	go func() {
		for {
			select {
			case <-terminate:
				wp.wait.Done()

				return
			case firstCall := <-calls:
				batchTask := append(make([]Task, 0), firstCall)

				collectionTimeout := time.After(wp.options.BatchCollectionTimeout)

				// collect additional requests that arrive within the timeout
			CollectAdditionalCalls:
				for {
					select {
					case <-terminate:
						wp.wait.Done()

						return
					case <-collectionTimeout:
						break CollectAdditionalCalls
					case call := <-wp.calls:
						batchTask = append(batchTask, call)

						if len(batchTask) == wp.options.BatchSize {
							break CollectAdditionalCalls
						}
					}
				}

				wp.batchedCalls <- batchTask
			}
		}
	}()
}

func (wp *BatchWorkerPool) startBatchWorkers() {
	batchedCalls := wp.batchedCalls
	terminate := wp.terminate

	for i := 0; i < wp.options.WorkerCount; i++ {
		wp.wait.Add(1)

		go func() {
			aborted := false

			for !aborted {
				select {
				case <-terminate:
					aborted = true

				case batchTask := <-batchedCalls:
					wp.workerFnc(batchTask)
				}
			}

			wp.wait.Done()
		}()
	}
}
