package fpc

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc/instances"
)

var PLUGIN = node.NewPlugin("FPC", configure, run)

func configure(plugin *node.Plugin) {
	instances.Configure(plugin)
}

func run(plugin *node.Plugin) {
	instances.Run(plugin)
}
