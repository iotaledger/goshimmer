package chat

import (
	"time"

	"github.com/iotaledger/hive.go/generics/event"
)

// Events define events occurring within a Chat.
type Events struct {
	MessageReceived *event.Event[*MessageReceivedEvent]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		MessageReceived: event.New[*MessageReceivedEvent](),
	}
}

// Event defines the information passed when a chat event fires.
type MessageReceivedEvent struct {
	From      string
	To        string
	Message   string
	Timestamp time.Time
	MessageID string
}
