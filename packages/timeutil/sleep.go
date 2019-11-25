package timeutil

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
)

func Sleep(interval time.Duration) bool {
	select {
	case <-daemon.ShutdownSignal:
		return false

	case <-time.After(interval):
		return true
	}
}
