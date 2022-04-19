package drng

import (
	"testing"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/clock"
)

func TestCollectiveBeaconEvent(t *testing.T) {
	var cbReceived *CollectiveBeaconEvent

	eventTest := event.New[*CollectiveBeaconEvent]()

	eventTest.Attach(event.NewClosure(func(cb *CollectiveBeaconEvent) {
		cbReceived = cb
	}))

	cbTriggered := &CollectiveBeaconEvent{
		Timestamp: clock.SyncedTime(),
	}
	eventTest.Trigger(cbTriggered)

	require.Equal(t, cbTriggered, cbReceived)
}
