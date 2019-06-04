package datastructure

import (
	"testing"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(5)

	cache.Contains("test", func(elem interface{}, contains bool) {
		if !contains {
			cache.Set("test", 12)
		}
	})

	if !cache.Contains("test", func(elem interface{}, contains bool) {
		if !contains || elem != 12 {
			t.Error("the cache contains the wrong element")
		}
	}) {
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

	if cache.Contains("test") {
		t.Error("'test' should have been dropped")
	}

	cache.Set("a", 6)
	cache.Set("f", 8)

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	if !cache.Contains("a") {
		t.Error("'a' should not have been dropped")
	}
	if cache.Contains("b") {
		t.Error("'b' should have been dropped")
	}

	cache.Contains("tust", func(elem interface{}, contains bool) {
		if !contains {
			cache.Set("tust", 1337)
		}
	})

	if cache.GetSize() != 5 {
		t.Error("the size should be 5")
	}

	cache.Contains("a", func(value interface{}, exists bool) {
		if exists {
			cache.Delete("a")
		}
	})
	if cache.GetSize() != 4 {
		t.Error("the size should be 4")
	}

	cache.Delete("f")
	if cache.GetSize() != 3 {
		t.Error("the size should be 3")
	}
}

func BenchmarkLRUCache(b *testing.B) {
	cache := NewLRUCache(10000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Contains(i, func(val interface{}, exists bool) {
			if !exists {
				cache.Set(i, i)
			}
		})
	}
}
