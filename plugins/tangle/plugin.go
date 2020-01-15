package tangle

import (
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", node.Enabled, configure, run)
var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger("Tangle")

	configureTransactionDatabase()
	configureTransactionMetaDataDatabase()
	configureApproversDatabase()
	configureBundleDatabase()
	configureTransactionHashesForAddressDatabase()
	configureSolidifier()

	daemon.BackgroundWorker("Cache Flush", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		log.Info("Flushing caches to database...")
		FlushTransactionCache()
		FlushTransactionMetadata()
		FlushApproversCache()
		FlushBundleCache()
		log.Info("Flushing caches to database... done")

		log.Info("Syncing database to disk...")
		database.GetBadgerInstance().Close()
		log.Info("Syncing database to disk... done")
	}, shutdown.ShutdownPriorityTangle)

}

func run(*node.Plugin) {
	runSolidifier()
}

// Requester provides the functionality to request a transaction from the network.
type Requester interface {
	RequestTransaction(hash trinary.Hash)
}

type RequesterFunc func(hash trinary.Hash)

func (f RequesterFunc) RequestTransaction(hash trinary.Hash) { f(hash) }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
