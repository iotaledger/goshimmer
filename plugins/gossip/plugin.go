package gossip

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Gossip", node.Enabled, configure, run)
var log = logger.NewLogger("Gossip")

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
