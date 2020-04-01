package server

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
)

const (
	IDLE_TIMEOUT = 10 * time.Second

	STATE_HEARTBEAT = heartbeat.MARSHALED_PACKET_HEADER
)
