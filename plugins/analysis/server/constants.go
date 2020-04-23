package server

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
)

const (
	IdleTimeout = 10 * time.Second

	StateHeartbeat = heartbeat.MarshaledPacketHeader
)
