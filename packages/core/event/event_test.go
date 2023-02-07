package event

import (
	"testing"
)

func Benchmark(b *testing.B) {
	testEvent := New1[int]()
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}
