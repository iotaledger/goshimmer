package clock

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// CfgNTPPools defines the config flag of the NTP pools.
	CfgNTPPools = "clock.ntpPools"

	// PluginName is the name of the clock plugin.
	PluginName = "Clock"

	maxTries = 3
)

var (
	plugin     *node.Plugin
	pluginOnce sync.Once
	log        *logger.Logger
	ntpPools   []string

	// ErrSynchronizeClock is used when the local clock could not be synchronized.
	ErrSynchronizeClock = errors.New("could not synchronize clock")
)

// Plugin gets the clock plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func init() {
	flag.StringSlice(CfgNTPPools, []string{"0.pool.ntp.org", "1.pool.ntp.org", "2.pool.ntp.org"}, "list of NTP pools to synchronize time from")
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)

	ntpPools = config.Node().Strings(CfgNTPPools)
	if len(ntpPools) == 0 {
		log.Fatalf("%s needs to provide at least 1 NTP pool to synchronize the local clock.", CfgNTPPools)
	}
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		// sync clock on startup
		queryNTPPool()

		// sync clock every hour to counter drift
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownSignal:
				return
			case <-ticker.C:
				queryNTPPool()
			}
		}
	}, shutdown.PrioritySynchronization); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// queryNTPPool queries configured ntpPools for maxTries.
// If clock could not be successfully synchronized shuts down node gracefully.
func queryNTPPool() {
	log.Info("Synchronizing clock...")
	ok := false
	for t := maxTries; t > 0; t-- {
		index := rand.Int() % len(ntpPools)
		err := clock.FetchTimeOffset(ntpPools[index])
		if err == nil {
			ok = true
			break
		}

		log.Warn("error while trying to sync clock")
	}

	if ok {
		log.Info("Synchronizing clock... done")
	} else {
		gracefulshutdown.ShutdownWithError(ErrSynchronizeClock)
	}
}
