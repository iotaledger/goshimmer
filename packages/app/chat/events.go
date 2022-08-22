package chat

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events define events occurring within a Chat.
type Events struct {
	BlockReceived *event.Event[*BlockReceivedEvent]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		BlockReceived: event.New[*BlockReceivedEvent](),
	}
}

// BlockReceivedEvent defines the information passed when a chat event fires.
type BlockReceivedEvent struct {
	From      string
	To        string
	Block     string
	Timestamp time.Time
	BlockID   string
}
