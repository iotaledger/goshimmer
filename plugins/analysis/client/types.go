package client

import "github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"

// EventDispatchers holds the Heartbeat function.
type EventDispatchers struct {
	// Heartbeat defines the Heartbeat function.
	Heartbeat func(*heartbeat.Packet)
}
