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

const (
	PLUGIN_NAME = "Database"
)

var (
	PLUGIN = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)

	_ = database.GetBadgerInstance()
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker(PLUGIN_NAME+"_GC", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {
			database.CleanupBadgerInstance(log)
		}, 5*time.Minute, shutdownSignal)
	}, shutdown.ShutdownPriorityBadgerGarbageCollection)

	daemon.BackgroundWorker(PLUGIN_NAME, func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		log.Infof("Syncing database to disk...")
		database.GetBadgerInstance().Close()
		log.Infof("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityDatabase)
}
