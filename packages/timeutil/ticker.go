package timeutil

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
)

func Ticker(handler func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
ticker:
	for {
		select {
		case <-daemon.ShutdownSignal:
			break ticker
		case <-ticker.C:
			handler()
		}
	}
}
