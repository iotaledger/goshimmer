package workerpool

import (
	"sync"
)

type WorkerPool struct {
	workerFnc func(Task)
	options   *Options

	calls     chan Task
	terminate chan int

	running bool
	mutex   sync.RWMutex
	wait    sync.WaitGroup
}

func New(workerFnc func(Task), optionalOptions ...Option) (result *WorkerPool) {
	options := DEFAULT_OPTIONS.Override(optionalOptions...)

	result = &WorkerPool{
		workerFnc: workerFnc,
		options:   options,
	}

	result.resetChannels()

	return
}

func (wp *WorkerPool) Submit(params ...interface{}) (result chan interface{}) {
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

func (wp *WorkerPool) Start() {
	wp.mutex.Lock()

	if !wp.running {
		wp.running = true

		wp.startWorkers()
	}

	wp.mutex.Unlock()
}

func (wp *WorkerPool) Run() {
	wp.Start()

	wp.wait.Wait()
}

func (wp *WorkerPool) Stop() {
	go wp.StopAndWait()
}

func (wp *WorkerPool) StopAndWait() {
	wp.mutex.Lock()

	if wp.running {
		wp.running = false

		close(wp.terminate)
		wp.resetChannels()
	}

	wp.wait.Wait()

	wp.mutex.Unlock()
}

func (wp *WorkerPool) resetChannels() {
	wp.calls = make(chan Task, wp.options.QueueSize)
	wp.terminate = make(chan int, 1)
}

func (wp *WorkerPool) startWorkers() {
	calls := wp.calls
	terminate := wp.terminate

	for i := 0; i < wp.options.WorkerCount; i++ {
		wp.wait.Add(1)

		go func() {
			aborted := false

			for !aborted {
				select {
				case <-terminate:
					aborted = true

				case batchTask := <-calls:
					wp.workerFnc(batchTask)
				}
			}

			wp.wait.Done()
		}()
	}
}
