package fpc

import (
	"github.com/iotaledger/goshimmer/packages/node"
)

var PLUGIN = node.NewPlugin("FPC", configure, run)

func configure(plugin *node.Plugin) {
	configureFPC(plugin)
}

func run(plugin *node.Plugin) {
	runFPC(plugin)
}
