package clock

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

const (
	maxTries     = 3
	syncInterval = 30 * time.Minute
)

var (
	plugin     *node.Plugin
	pluginOnce sync.Once
)

// Plugin gets the clock plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("Clock", node.Enabled, configure, run)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	if len(Parameters.NTPPools) == 0 {
		Plugin().LogFatalf("at least 1 NTP pool needs to be provided to synchronize the local clock.")
	}
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(Plugin().Name, func(shutdownSignal <-chan struct{}) {
		// sync clock on startup
		queryNTPPool()

		// sync clock every 30min to counter drift
		timeutil.NewTicker(queryNTPPool, syncInterval, shutdownSignal)

		<-shutdownSignal
	}, shutdown.PrioritySynchronization); err != nil {
		Plugin().Panicf("Failed to start as daemon: %s", err)
	}
}

// queryNTPPool queries configured ntpPools for maxTries.
func queryNTPPool() {
	Plugin().LogDebug("Synchronizing clock...")
	for t := maxTries; t > 0; t-- {
		index := rand.Int() % len(Parameters.NTPPools)
		err := clock.FetchTimeOffset(Parameters.NTPPools[index])
		if err == nil {
			Plugin().LogDebug("Synchronizing clock... done")
			return
		}
	}

	Plugin().LogWarn("error while trying to sync clock")
}
