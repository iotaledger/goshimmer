package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/plugins/analysis"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/cli"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
	"github.com/iotaledger/goshimmer/plugins/logger"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/goshimmer/plugins/portcheck"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/spa"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	webapi_broadcastData "github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi/gtta"
	webapi_spammer "github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	webapi_auth "github.com/iotaledger/goshimmer/plugins/webauth"

	"github.com/iotaledger/hive.go/node"
)

func main() {
	go http.ListenAndServe("localhost:6061", nil) // pprof Server for Debbuging Mutexes

	testTxId := transaction.NewId([]byte("Test"))

	fmt.Println(len(base58.Encode(transaction.EmptyId[:])))
	fmt.Println(base58.Encode(transaction.EmptyId[:]))
	fmt.Println(len(base58.Encode(testTxId[:])))
	fmt.Println(base58.Encode(testTxId[:]))

	node.Run(
		node.Plugins(
			banner.PLUGIN,
			config.PLUGIN,
			logger.PLUGIN,
			cli.PLUGIN,
			remotelog.PLUGIN,
			portcheck.PLUGIN,

			autopeering.PLUGIN,
			tangle.PLUGIN,
			gossip.PLUGIN,
			gracefulshutdown.PLUGIN,

			analysis.PLUGIN,
			metrics.PLUGIN,

			webapi.PLUGIN,
			webapi_auth.PLUGIN,
			webapi_gtta.PLUGIN,
			webapi_spammer.PLUGIN,
			webapi_broadcastData.PLUGIN,

			spa.PLUGIN,

			/*
				webapi_getTransactionTrytesByHash.PLUGIN,
				webapi_getTransactionObjectsByHash.PLUGIN,
				webapi_findTransactionHashes.PLUGIN,
				webapi_getNeighbors.PLUGIN,

				//graph.PLUGIN,
			*/
		),
	)
}
