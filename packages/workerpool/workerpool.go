package workerpool

import (
	"fmt"
)

type WorkerPool struct {
	maxWorkers   int
	MaxQueueSize int
	callsChan    chan Call
}

func New(maxWorkers int) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
	}
}

type Call struct {
	params     []interface{}
	resultChan chan interface{}
}

func (wp *WorkerPool) Submit(params ...interface{}) (result chan interface{}) {
	result = make(chan interface{}, 1)

	wp.callsChan <- Call{
		params:     params,
		resultChan: result,
	}

	return
}

func (wp *WorkerPool) startWorkers() {
	for i := 0; i < wp.maxWorkers; i++ {
		go func() {
			for {
				batchTasks := <-wp.callsChan

				fmt.Println(batchTasks)
			}
		}()
	}
}

func (wp *WorkerPool) Start() {
	wp.callsChan = make(chan Call, 2*wp.maxWorkers)

	wp.startWorkers()
}
