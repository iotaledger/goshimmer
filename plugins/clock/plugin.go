package clock

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

const (
	maxTries     = 3
	syncInterval = 30 * time.Minute
)

var (
	Plugin     *node.Plugin
	pluginOnce sync.Once
)

func init() {
	Plugin = node.NewPlugin("Clock", node.Enabled, configure, run)

	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := dependencyinjection.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("clock")); err != nil {
			panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	if len(Parameters.NTPPools) == 0 {
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
	Plugin.LogDebug("Synchronizing clock...")
	for t := maxTries; t > 0; t-- {
		index := rand.Int() % len(Parameters.NTPPools)
		err := clock.FetchTimeOffset(Parameters.NTPPools[index])
		if err == nil {
			Plugin.LogDebug("Synchronizing clock... done")
			return
		}
	}

	Plugin.LogWarn("error while trying to sync clock")
}
