package gossip

import "github.com/iotaledger/goshimmer/packages/node"

var PLUGIN = node.NewPlugin("Gossip", configure, run)

func configure(plugin *node.Plugin) {
    configureServer(plugin)
}

func run(plugin *node.Plugin) {
    runServer(plugin)
}
