package timeutil

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
)

func Sleep(interval time.Duration) bool {
	select {
	case <-daemon.ShutdownSignal:
		return false

	case <-time.After(interval):
		return true
	}
}
