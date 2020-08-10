package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	cbEvents "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectivebeacon/events"
	"github.com/iotaledger/hive.go/events"
)

// DRNG holds the state and events of a drng instance.
type DRNG struct {
	State  *state.State // The state of the DRNG.
	Events *Event       // The events fired on the DRNG.
}

// New creates a new DRNG instance.
func New(setters ...state.Option) *DRNG {
	return &DRNG{
		State: state.New(setters...),
		Events: &Event{
			CollectiveBeacon: events.NewEvent(cbEvents.CollectiveBeaconReceived),
			Randomness:       events.NewEvent(randomnessReceived),
		},
	}
}
