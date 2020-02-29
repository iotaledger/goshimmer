package orderedmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedMap_Size(t *testing.T) {
	orderedMap := New()

	assert.Equal(t, 0, orderedMap.Size())

	orderedMap.Set(1, 1)

	assert.Equal(t, 1, orderedMap.Size())

	orderedMap.Set(3, 1)
	orderedMap.Set(2, 1)

	assert.Equal(t, 3, orderedMap.Size())

	orderedMap.Set(2, 2)

	assert.Equal(t, 3, orderedMap.Size())

	orderedMap.Delete(2)

	assert.Equal(t, 2, orderedMap.Size())
}
