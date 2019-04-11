package tcp

import "time"

const (
    IDLE_TIMEOUT = 5 * time.Second

    STATE_INITIAL  = byte(0)
    STATE_REQUEST  = byte(1)
    STATE_RESPONSE = byte(2)
    STATE_PING     = byte(3)
)
