package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/logger"

	"github.com/iotaledger/goshimmer/plugins/tangle"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/node"
)

func main() {
	go http.ListenAndServe("localhost:6060", nil) // pprof Server for Debbuging Mutexes

	node.Run(
		node.Plugins(
			config.PLUGIN,
			logger.PLUGIN,
			cli.PLUGIN,
			remotelog.PLUGIN,

			autopeering.PLUGIN,
			tangle.PLUGIN,
			gossip.PLUGIN,
			gracefulshutdown.PLUGIN,

			/*
				analysis.PLUGIN,
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

				//spa.PLUGIN,
				//graph.PLUGIN,
			*/
		),
	)
}
