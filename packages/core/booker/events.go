package booker

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	BlockBooked *event.Event[*Block]
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockBooked: event.New[*Block](),
	}
}
