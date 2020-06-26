package orderedmap

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNew(t *testing.T) {
	orderedMap := New()
	require.NotNil(t, orderedMap)

	assert.Equal(t, 0, orderedMap.Size())

	assert.Nil(t, orderedMap.head)
	assert.Nil(t, orderedMap.tail)
}

func TestSetGetDelete(t *testing.T) {
	orderedMap := New()
	require.NotNil(t, orderedMap)

	// when adding the first new key,value pair, we must return true
	keyValueAdded := orderedMap.Set("key", "value")
	assert.True(t, keyValueAdded)

	// we should be able to retrieve the just added element
	value, ok := orderedMap.Get("key")
	assert.Equal(t, "value", value)
	assert.True(t, ok)

	// head and tail should NOT be nil and match and size should be 1
	assert.NotNil(t, orderedMap.head)
	assert.Same(t, orderedMap.head, orderedMap.tail)
	assert.Equal(t, 1, orderedMap.Size())

	// when adding the same key,value pair must return false
	// and size should not change;
	keyValueAdded = orderedMap.Set("key", "value")
	assert.False(t, keyValueAdded)
	assert.Equal(t, 1, orderedMap.Size())

	// when retrieving something that does not exist we
	// should get nil, false
	value, ok = orderedMap.Get("keyNotStored")
	assert.Nil(t, value)
	assert.False(t, ok)

	// when deleting an existing element, we must get true,
	// the element must be removed, and size decremented.
	deleted := orderedMap.Delete("key")
	assert.True(t, deleted)
	value, ok = orderedMap.Get("key")
	assert.Nil(t, value)
	assert.False(t, ok)
	assert.Equal(t, 0, orderedMap.Size())

	// if we delete the only element, head and tail should be both nil
	assert.Nil(t, orderedMap.head)
	assert.Same(t, orderedMap.head, orderedMap.tail)

	// when deleting a NON existing element, we must get false
	deleted = orderedMap.Delete("key")
	assert.False(t, deleted)
}

func TestForEach(t *testing.T) {
	orderedMap := New()
	require.NotNil(t, orderedMap)

	testElements := []Element{
		{key: "one", value: 1},
		{key: "two", value: 2},
		{key: "three", value: 3},
	}

	for _, element := range testElements {
		keyValueAdded := orderedMap.Set(element.key, element.value)
		assert.True(t, keyValueAdded)
	}

	// test that all elements are positive via ForEach
	testPositive := orderedMap.ForEach(func(key, value interface{}) bool {
		return value.(int) > 0
	})
	assert.True(t, testPositive)

	testNegative := orderedMap.ForEach(func(key, value interface{}) bool {
		return value.(int) < 0
	})
	assert.False(t, testNegative)
}

func TestConcurrencySafe(t *testing.T) {
	orderedMap := New()
	require.NotNil(t, orderedMap)

	// initialize a slice of 100 elements
	set := make([]Element, 100)
	for i := 0; i < 100; i++ {
		element := Element{key: fmt.Sprintf("%d", i), value: i}
		set[i] = element
	}

	// let 10 workers fill the orderedMap
	workers := 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				ele := set[i]
				orderedMap.Set(ele.key, ele.value)
			}
		}()
	}
	wg.Wait()

	// check that all the elements consumed from the set
	// have been stored in the orderedMap and its size matches
	for i := 0; i < 100; i++ {
		value, ok := orderedMap.Get(set[i].key)
		assert.Equal(t, set[i].value, value)
		assert.True(t, ok)
	}
	assert.Equal(t, 100, orderedMap.Size())

	// let 10 workers delete elements from the orderedMAp
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				ele := set[i]
				orderedMap.Delete(ele.key)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, orderedMap.Size())
}
