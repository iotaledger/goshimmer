package messagerequester

import (
	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening on a message requester.
type Events struct {
	// Fired when a request for a given message should be sent.
	SendRequest *events.Event
}
