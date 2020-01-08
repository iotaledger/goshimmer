package tangle

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", node.Enabled, configure, run)
var log = logger.NewLogger("Tangle")

func configure(*node.Plugin) {
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
