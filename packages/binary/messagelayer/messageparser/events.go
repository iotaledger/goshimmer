package messageparser

import "github.com/iotaledger/hive.go/events"

// Events represents events happening on a message parser.
type Events struct {
	// Fired when a message was parsed.
	MessageParsed *events.Event
	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *events.Event
	// Fired when a message got rejected by a filter.
	MessageRejected *events.Event
}
