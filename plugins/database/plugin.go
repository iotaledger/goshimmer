// database is a plugin that manages the badger database (e.g. garbage collection).
package database

import (
	"time"

	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the database plugin.
const PluginName = "Database"

var (
	// Plugin is the plugin instance of the database plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	_ = database.GetBadgerInstance()
}

func run(_ *node.Plugin) {
	daemon.BackgroundWorker(PluginName+"_GC", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {
			database.CleanupBadgerInstance(log)
		}, 5*time.Minute, shutdownSignal)
	}, shutdown.PriorityBadgerGarbageCollection)

	daemon.BackgroundWorker(PluginName, func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		log.Infof("Syncing database to disk...")
		database.GetBadgerInstance().Close()
		log.Infof("Syncing database to disk... done")
	}, shutdown.PriorityDatabase)
}
