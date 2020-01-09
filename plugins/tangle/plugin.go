package tangle

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
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
	configureSolidifier()
}

func run(*node.Plugin) {
	runSolidifier()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
