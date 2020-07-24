package messagerequester

import (
	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening on a message requester.
type Events struct {
	// Fired when a request for a given message should be sent.
	SendRequest *events.Event
	// MissingMessageAppeared is triggered when a message is actually present in the node's db although it was still being requested.
	MissingMessageAppeared *events.Event
}
