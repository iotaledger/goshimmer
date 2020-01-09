package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/bundleprocessor"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/statusscreen"
	statusscreen_tps "github.com/iotaledger/goshimmer/plugins/statusscreen-tps"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/ui"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	webapi_broadcastData "github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi/gtta"
	webapi_spammer "github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	webapi_transaction "github.com/iotaledger/goshimmer/plugins/webapi/transaction"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/goshimmer/plugins/zeromq"
	"github.com/iotaledger/hive.go/node"
)

func main() {
	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes

	node.Run(
		node.Plugins(
			cli.PLUGIN,
			autopeering.PLUGIN,
			gossip.PLUGIN,
			tangle.PLUGIN,
			bundleprocessor.PLUGIN,
			analysis.PLUGIN,
			gracefulshutdown.PLUGIN,
			tipselection.PLUGIN,
			zeromq.PLUGIN,
			dashboard.PLUGIN,
			metrics.PLUGIN,

			statusscreen.PLUGIN,
			statusscreen_tps.PLUGIN,

			webapi.PLUGIN,
			webapi_gtta.PLUGIN,
			webapi_spammer.PLUGIN,
			webapi_broadcastData.PLUGIN,
			webapi_transaction.PLUGIN,

			ui.PLUGIN,
			webauth.PLUGIN,
		),
	)
}
