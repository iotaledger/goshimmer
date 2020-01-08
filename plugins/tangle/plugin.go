package tangle

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", node.Enabled, configure, run)
var log *logger.Logger

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Tangle")
	configureTransactionDatabase(plugin)
	configureTransactionMetaDataDatabase(plugin)
	configureApproversDatabase(plugin)
	configureBundleDatabase(plugin)
	configureSolidifier(plugin)
}

func run(plugin *node.Plugin) {
	runSolidifier(plugin)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
