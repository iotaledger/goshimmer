package tipmanager

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/hive.go/events"
)

// Events represents events happening on the TipManager.
type Events struct {
	// Fired when a tip is added.
	TipAdded *events.Event
	// Fired when a tip is removed.
	TipRemoved *events.Event
}

func payloadIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.ID))(params[0].(payload.ID))
}
