package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	BlockTracked *event.Event[*Block]
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockTracked: event.New[*Block](),
	}
}
