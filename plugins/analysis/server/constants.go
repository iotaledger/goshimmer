package server

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
)

const (
	// IdleTimeout defines the idle timeout.
	IdleTimeout = 10 * time.Second
	// StateHeartbeat defines the state of the heartbeat.
	StateHeartbeat = heartbeat.MarshaledPacketHeader
)
