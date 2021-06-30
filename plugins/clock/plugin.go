package clock

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
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
	if len(Parameters.NtpPools) == 0 {
		plugin.LogFatalf("at least 1 NTP pool needs to be provided to synchronize the local clock.")
	}
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(plugin.Name, func(shutdownSignal <-chan struct{}) {
		// sync clock on startup
		queryNTPPool()

		// sync clock every 30min to counter drift
		timeutil.NewTicker(queryNTPPool, syncInterval, shutdownSignal)

		<-shutdownSignal
	}, shutdown.PrioritySynchronization); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

// queryNTPPool queries configured ntpPools for maxTries.
func queryNTPPool() {
	plugin.LogDebug("Synchronizing clock...")
	for t := maxTries; t > 0; t-- {
		index := rand.Int() % len(Parameters.NtpPools)
		err := clock.FetchTimeOffset(Parameters.NtpPools[index])
		if err == nil {
			plugin.LogDebug("Synchronizing clock... done")
			return
		}
	}

	plugin.LogWarn("error while trying to sync clock")
}
