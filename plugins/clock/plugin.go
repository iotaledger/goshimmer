package clock

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
)

const (
	// CfgNTPPools defines the config flag of the NTP pools.
	CfgNTPPools = "clock.ntpPools"

	maxTries     = 3
	syncInterval = 30 * time.Minute
)

var (
	plugin     *node.Plugin
	pluginOnce sync.Once
	ntpPools   []string
)

// Plugin gets the clock plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("Clock", node.Enabled, configure, run)
	})
	return plugin
}

func init() {
	flag.StringSlice(CfgNTPPools, []string{"0.pool.ntp.org", "1.pool.ntp.org", "2.pool.ntp.org"}, "list of NTP pools to synchronize time from")
}

func configure(plugin *node.Plugin) {
	ntpPools = config.Node().Strings(CfgNTPPools)
	if len(ntpPools) == 0 {
		plugin.LogFatalf("%s needs to provide at least 1 NTP pool to synchronize the local clock.", CfgNTPPools)
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
		index := rand.Int() % len(ntpPools)
		err := clock.FetchTimeOffset(ntpPools[index])
		if err == nil {
			plugin.LogDebug("Synchronizing clock... done")
			return
		}
	}

	plugin.LogWarn("error while trying to sync clock")
}
