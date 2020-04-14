package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/hive.go/events"
)

// Event holds the different events triggered by a DRNG instance.
type Event struct {
	// Collective Beacon is triggered each time we receive a new CollectiveBeacon message.
	CollectiveBeacon *events.Event
	// Randomness is triggered each time we receive a new and valid CollectiveBeacon message.
	Randomness *events.Event
}

func randomnessReceived(handler interface{}, params ...interface{}) {
	handler.(func(state.Randomness))(params[0].(state.Randomness))
}
