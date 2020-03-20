package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	cbEvents "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	"github.com/iotaledger/hive.go/events"
)

type Event struct {
	CollectiveBeacon cbEvents.CollectiveBeacon
	Randomness       *events.Event
}

func NewEvent() *Event {
	return &Event{
		CollectiveBeacon: cbEvents.NewCollectiveBeaconEvent(),
		Randomness:       events.NewEvent(randomnessReceived),
	}
}

func randomnessReceived(handler interface{}, params ...interface{}) {
	handler.(func(*state.Randomness))(params[0].(*state.Randomness))
}
