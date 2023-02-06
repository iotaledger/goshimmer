package event

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/event/generic"
)

func Benchmark(b *testing.B) {
	testEvent := generic.New1[int]()
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})
	testEvent.Hook(func(int) {})

	for i := 0; i < b.N; i++ {
		testEvent.Trigger(i)
	}
}

//
// func Benchmark2(b *testing.B) {
// 	testEvent := New2[int, int]()
// 	testEvent.Hook(func(int, int) {})
// 	testEvent.Hook(func(int, int) {})
// 	testEvent.Hook(func(int, int) {})
// 	testEvent.Hook(func(int, int) {})
//
// 	for i := 0; i < b.N; i++ {
// 		testEvent.Trigger(i, i)
// 	}
// }

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
