package queue

import (
	"sync"
)

type Queue struct {
	ringBuffer []interface{}
	read       int
	write      int
	capacity   int
	size       int
	mutex      sync.Mutex
}

func New(capacity int) *Queue {
	return &Queue{
		ringBuffer: make([]interface{}, capacity),
		capacity:   capacity,
	}
}

func (queue *Queue) Size() int {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	return queue.size
}

func (queue *Queue) Capacity() int {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	return queue.capacity
}

func (queue *Queue) Offer(element interface{}) bool {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	if queue.size == queue.capacity {
		return false
	}

	queue.ringBuffer[queue.write] = element
	queue.write = (queue.write + 1) % queue.capacity
	queue.size++

	return true
}

func (queue *Queue) Poll() (element interface{}, success bool) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	if success = queue.size != 0; !success {
		return
	}

	element = queue.ringBuffer[queue.read]
	queue.ringBuffer[queue.read] = nil
	queue.read = (queue.read + 1) % queue.capacity
	queue.size--

	return
}
