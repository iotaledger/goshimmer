package webapi

import (
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

var PLUGINS = node.Plugins(
	webapi.PLUGIN,
	webapi_auth.PLUGIN,
	webapi_gtta.PLUGIN,
	webapi_spammer.PLUGIN,
	webapi_broadcastData.PLUGIN,
	webapi_getTransactionTrytesByHash.PLUGIN,
	webapi_getTransactionObjectsByHash.PLUGIN,
	webapi_findTransactionHashes.PLUGIN,
	webapi_getNeighbors.PLUGIN,
)
