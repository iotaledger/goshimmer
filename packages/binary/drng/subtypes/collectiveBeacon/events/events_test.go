package events

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/require"
)

func TestCollectiveBeaconEvent(t *testing.T) {
	var cbReceived *CollectiveBeaconEvent

	eventTest := NewCollectiveBeaconEvent()

	eventTest.Attach(events.NewClosure(func(cb *CollectiveBeaconEvent) {
		cbReceived = cb
	}))

	cbTriggered := &CollectiveBeaconEvent{
		Timestamp: time.Now(),
	}
	eventTest.Trigger(cbTriggered)

	require.Equal(t, cbTriggered, cbReceived)

}
