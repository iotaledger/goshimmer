package client

import "github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"

type EventDispatchers struct {
	Heartbeat func(*heartbeat.Packet)
}
