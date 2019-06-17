package datastructure

import (
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(5)

	cache.ComputeIfAbsent("test", func() interface{} {
		return 12
	})

	if cache.Get("test") != 12 {
		t.Error("the cache does not contain the added elements")
	}

	if cache.GetSize() != 1 {
		t.Error("the size should be 1")
	}

	if cache.GetCapacity() != 5 {
		t.Error("the capacity should be 5")
	}

	cache.Set("a", 3)
	cache.Set("b", 4)
	cache.Set("c", 5)
	cache.Set("d", 6)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	cache.Set("e", 7)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("test") != nil {
		t.Error("'test' should have been dropped")
	}

	cache.Set("a", 6)
	cache.Set("f", 8)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("a") == nil {
		t.Error("'a' should not have been dropped")
	}
	if cache.Get("b") != nil {
		t.Error("'b' should have been dropped")
	}

	{
		key, value := "test2", 1337

		cache.ComputeIfAbsent(key, func() interface{} {
			return value
		})
		if cache.Get(key) != value {
			t.Error("'" + key + "' should have been added")
		}
	}

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if cache.Get("a") != nil {
		cache.Delete("a")
	}
	if cache.GetSize() != 4 {
		t.Error("the size should be 4")
	}

	cache.Delete("f")
	if cache.GetSize() != 3 {
		t.Error("the size should be 3")
	}
}

func TestLRUCache_ComputeIfPresent(t *testing.T) {
	cache := NewLRUCache(5)
	cache.Set(8, 9)

	cache.ComputeIfPresent(8, func(value interface{}) interface{} {
		return 88
	})
	if cache.Get(8) != 88 || cache.GetSize() != 1 {
		t.Error("cache was not updated correctly")
	}

	cache.ComputeIfPresent(8, func(value interface{}) interface{} {
		return nil
	})
	if cache.Get(8) != nil || cache.GetSize() != 0 {
		t.Error("cache was not updated correctly")
	}
}
