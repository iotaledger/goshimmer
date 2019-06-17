package gossip

import (
	"github.com/iotaledger/goshimmer/packages/node"
)

var PLUGIN = node.NewPlugin("Gossip", configure, run)

func configure(plugin *node.Plugin) {
	configureNeighbors(plugin)
	configureServer(plugin)
	configureSendQueue(plugin)
}

func run(plugin *node.Plugin) {
	runNeighbors(plugin)
	runServer(plugin)
	runSendQueue(plugin)
}
