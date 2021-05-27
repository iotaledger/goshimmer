package chat

import (
	"time"

	"github.com/iotaledger/hive.go/events"
)

// Events defines the events of the plugin.
var Events = pluginEvents{
	// ReceivedChatMessage triggers upon reception of a chat message.
	MessageReceived: events.NewEvent(chatEventCaller),
}

type pluginEvents struct {
	// Fired when a chat message is received.
	MessageReceived *events.Event
}

// ChatEvent defines the information passed when a chat event fires.
type ChatEvent struct {
	From      string
	To        string
	Message   string
	Timestamp time.Time
	MessageID string
}

func chatEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*ChatEvent))(params[0].(*ChatEvent))
}
