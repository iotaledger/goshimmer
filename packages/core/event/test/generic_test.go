package test

import (
	"testing"

	old "github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/event"
)

func Benchmark(b *testing.B) {
	testEvent := event.New1[int]()
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}

func BenchmarkOldEvent(b *testing.B) {
	testEvent := old.NewLinkable[int]()
	testEvent.Hook(old.NewClosure(func(event int) {}))
	testEvent.Hook(old.NewClosure(func(event int) {}))
	testEvent.Hook(old.NewClosure(func(event int) {}))
	testEvent.Hook(old.NewClosure(func(event int) {}))

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}
