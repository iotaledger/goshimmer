package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	queue := New(2)
	assert.Equal(t, 0, queue.Size())
	assert.Equal(t, 2, queue.Capacity())

	assert.Equal(t, true, queue.Offer(1))
	assert.Equal(t, 1, queue.Size())

	assert.Equal(t, true, queue.Offer(2))
	assert.Equal(t, 2, queue.Size())

	assert.Equal(t, false, queue.Offer(3))

	polledValue, ok := queue.Poll()
	assert.Equal(t, true, ok)
	assert.Equal(t, 1, polledValue)
	assert.Equal(t, 1, queue.Size())

	polledValue, ok = queue.Poll()
	assert.Equal(t, true, ok)
	assert.Equal(t, 2, polledValue)
	assert.Equal(t, 0, queue.Size())

	polledValue, ok = queue.Poll()
	assert.Equal(t, false, ok)
	assert.Equal(t, nil, polledValue)
	assert.Equal(t, 0, queue.Size())

	assert.Equal(t, true, queue.Offer(3))
	assert.Equal(t, 1, queue.Size())

	assert.Equal(t, true, queue.Offer(4))
	assert.Equal(t, 2, queue.Size())
}
