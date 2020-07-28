package queue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	queue := New(2)
	require.NotNil(t, queue)
	assert.Equal(t, 0, queue.Size())
	assert.Equal(t, 2, queue.Capacity())
}

func TestQueueOfferPoll(t *testing.T) {
	queue := New(2)
	require.NotNil(t, queue)

	// offer element to queue
	assert.True(t, queue.Offer(1))
	assert.Equal(t, 1, queue.Size())

	assert.True(t, queue.Offer(2))
	assert.Equal(t, 2, queue.Size())

	assert.False(t, queue.Offer(3))

	// Poll element from queue
	polledValue, ok := queue.Poll()
	assert.True(t, ok)
	assert.Equal(t, 1, polledValue)
	assert.Equal(t, 1, queue.Size())

	polledValue, ok = queue.Poll()
	assert.True(t, ok)
	assert.Equal(t, 2, polledValue)
	assert.Equal(t, 0, queue.Size())

	polledValue, ok = queue.Poll()
	assert.False(t, ok)
	assert.Nil(t, polledValue)
	assert.Equal(t, 0, queue.Size())

	// Offer the empty queue again
	assert.True(t, queue.Offer(3))
	assert.Equal(t, 1, queue.Size())
}

func TestQueueOfferConcurrencySafe(t *testing.T) {
	queue := New(100)
	require.NotNil(t, queue)

	// let 10 workers fill the queue
	workers := 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				queue.Offer(j)
			}
		}()
	}
	wg.Wait()

	// check that all the elements are offered
	assert.Equal(t, 100, queue.Size())

	counter := make([]int, 10)
	for i := 0; i < 100; i++ {
		value, ok := queue.Poll()
		assert.True(t, ok)
		counter[value.(int)]++
	}
	assert.Equal(t, 0, queue.Size())

	// check that the insert numbers are correct
	for i := 0; i < 10; i++ {
		assert.Equal(t, 10, counter[i])
	}
}

func TestQueuePollConcurrencySafe(t *testing.T) {
	queue := New(100)
	require.NotNil(t, queue)

	for j := 0; j < 100; j++ {
		queue.Offer(j)
	}

	// let 10 workers poll the queue
	workers := 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, ok := queue.Poll()
				assert.True(t, ok)
			}
		}()
	}
	wg.Wait()

	// check that all the elements are polled
	assert.Equal(t, 0, queue.Size())
}
