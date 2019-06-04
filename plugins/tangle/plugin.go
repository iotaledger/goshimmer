package tangle

import (
	"github.com/iotaledger/goshimmer/packages/node"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", configure, run)

func configure(plugin *node.Plugin) {
	configureDatabase(plugin)
	configureMemPool(plugin)
}

func run(plugin *node.Plugin) {
	runMemPool(plugin)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
