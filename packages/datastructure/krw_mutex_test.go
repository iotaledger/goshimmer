package datastructure

import (
	"testing"
)

func TestKRWMutex_Free(t *testing.T) {
	krwMutex := NewKRWMutex()

	krwMutex.Register("test")
	krwMutex.Register("test")
	krwMutex.Free("test")
	krwMutex.Free("test")
}

func BenchmarkKRWMutex(b *testing.B) {
	krwMutex := NewKRWMutex()

	for i := 0; i < b.N; i++ {
		krwMutex.Register(i)
		krwMutex.Free(i)
	}
}
