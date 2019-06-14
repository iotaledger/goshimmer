package main

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/spammer"
	"github.com/iotaledger/goshimmer/plugins/statusscreen"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi-gtta"
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
		tipselection.PLUGIN,
		spammer.PLUGIN,

		webapi.PLUGIN,
		webapi_gtta.PLUGIN,
	)
}
