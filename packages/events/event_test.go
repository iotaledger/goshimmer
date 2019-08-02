package events

import (
	"fmt"
	"strconv"
	"testing"
)

func BenchmarkEvent_Trigger(b *testing.B) {
	event := NewEvent(intStringCaller)

	event.Attach(NewClosure(func(param1 int, param2 string) {
		// do nothing just get called
	}))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		event.Trigger(4, "test")
	}
}

// define how the event converts the generic parameters to the typed params - ugly but go has no generics :(
func intStringCaller(handler interface{}, params ...interface{}) {
	handler.(func(int, string))(params[0].(int), params[1].(string))
}

func ExampleEvent() {
	// create event object (usually exposed through a public struct that holds all the different event types)
	event := NewEvent(intStringCaller)

	// we have to wrap a function in a closure to make it identifiable
	closure1 := NewClosure(func(param1 int, param2 string) {
		fmt.Println("#1 " + param2 + ": " + strconv.Itoa(param1))
	})

	// multiple subscribers can attach to an event (closures can be inlined)
	event.Attach(closure1)
	event.Attach(NewClosure(func(param1 int, param2 string) {
		fmt.Println("#2 " + param2 + ": " + strconv.Itoa(param1))
	}))

	// trigger the event
	event.Trigger(1, "Hello World")

	// unsubscribe the first closure and trigger again
	event.Detach(closure1)
	event.Trigger(1, "Hello World")

	// Unordered output: #1 Hello World: 1
	// #2 Hello World: 1
	// #2 Hello World: 1
}
