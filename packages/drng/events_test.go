package drng

import (
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/clock"
)

func TestCollectiveBeaconEvent(t *testing.T) {
	var cbReceived *CollectiveBeaconEvent

	eventTest := events.NewEvent(CollectiveBeaconReceived)

	eventTest.Attach(events.NewClosure(func(cb *CollectiveBeaconEvent) {
		cbReceived = cb
	}))

	cbTriggered := &CollectiveBeaconEvent{
		Timestamp: clock.SyncedTime(),
	}
	eventTest.Trigger(cbTriggered)

	require.Equal(t, cbTriggered, cbReceived)
}
