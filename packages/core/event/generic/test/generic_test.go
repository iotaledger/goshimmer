package test

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/event/generic"
)

func Benchmark(b *testing.B) {
	testEvent := generic.NewWith1Param[int]()
	testEvent.Hook(func(int) {}, generic.WithWorkerPool(nil))
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}

func BenchmarkOldEvent(b *testing.B) {
	testEvent := event.NewLinkable[int]()
	testEvent.Hook(event.NewClosure(func(event int) {}))
	testEvent.Hook(event.NewClosure(func(event int) {}))
	testEvent.Hook(event.NewClosure(func(event int) {}))
	testEvent.Hook(event.NewClosure(func(event int) {}))

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}
