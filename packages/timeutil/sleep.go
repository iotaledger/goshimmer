package timeutil

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "time"
)

func Sleep(interval time.Duration) bool {
    select {
    case <-daemon.ShutdownSignal:
        return false

    case <-time.After(interval):
        return true
    }
}