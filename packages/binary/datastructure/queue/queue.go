package queue

import (
	"sync"
)

// Queue represents a ring buffer.
type Queue struct {
	ringBuffer []interface{}
	read       int
	write      int
	capacity   int
	size       int
	mutex      sync.Mutex
}

// New creates a new queue with the specified capacity.
func New(capacity int) *Queue {
	return &Queue{
		ringBuffer: make([]interface{}, capacity),
		capacity:   capacity,
	}
}

// Size returns the size of the queue.
func (queue *Queue) Size() int {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	return queue.size
}

// Capacity returns the capacity of the queue.
func (queue *Queue) Capacity() int {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	return queue.capacity
}

// Offer adds an element to the queue and returns true.
// If the queue is full, it drops it and returns false.
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

// Poll returns and removes the oldest element in the queue and true if successful.
// If returns false if the queue is empty.
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
