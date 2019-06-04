package main

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/statusscreen"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

func main() {
	node.Run(
		cli.PLUGIN,
		autopeering.PLUGIN,
		gossip.PLUGIN,
		tangle.PLUGIN,
		analysis.PLUGIN,
		statusscreen.PLUGIN,
		gracefulshutdown.PLUGIN,
	)
}
