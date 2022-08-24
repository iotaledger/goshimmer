package acceptancegadget

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	BlockAccepted *event.Event[*Block]
	Error         *event.Event[error]
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockAccepted: event.New[*Block](),
		Error:         event.New[error](),
	}
}
