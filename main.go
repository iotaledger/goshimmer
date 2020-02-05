package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/bundleprocessor"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/graph"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/spa"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	webapi_broadcastData "github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_findTransactionHashes "github.com/iotaledger/goshimmer/plugins/webapi/findTransactionHashes"
	webapi_getNeighbors "github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	webapi_getTransactionObjectsByHash "github.com/iotaledger/goshimmer/plugins/webapi/getTransactionObjectsByHash"
	webapi_getTransactionTrytesByHash "github.com/iotaledger/goshimmer/plugins/webapi/getTransactionTrytesByHash"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi/gtta"
	webapi_spammer "github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	webapi_auth "github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

func main() {
	cli.LoadConfig()

	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes

	node.Run(
		node.Plugins(
			cli.PLUGIN,
			remotelog.PLUGIN,

			autopeering.PLUGIN,
			gossip.PLUGIN,
			tangle.PLUGIN,
			bundleprocessor.PLUGIN,
			analysis.PLUGIN,
			gracefulshutdown.PLUGIN,
			tipselection.PLUGIN,
			metrics.PLUGIN,

			webapi.PLUGIN,
			webapi_auth.PLUGIN,
			webapi_gtta.PLUGIN,
			webapi_spammer.PLUGIN,
			webapi_broadcastData.PLUGIN,
			webapi_getTransactionTrytesByHash.PLUGIN,
			webapi_getTransactionObjectsByHash.PLUGIN,
			webapi_findTransactionHashes.PLUGIN,
			webapi_getNeighbors.PLUGIN,
			webapi_spammer.PLUGIN,

			spa.PLUGIN,
			graph.PLUGIN,
		),
	)
}
