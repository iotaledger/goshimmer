package webapi

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	"github.com/iotaledger/goshimmer/plugins/webapi/findTransactionHashes"
	"github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	"github.com/iotaledger/goshimmer/plugins/webapi/getTransactionObjectsByHash"
	"github.com/iotaledger/goshimmer/plugins/webapi/getTransactionTrytesByHash"
	"github.com/iotaledger/goshimmer/plugins/webapi/gtta"
	"github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	webapi.PLUGIN,
	webauth.PLUGIN,
	gtta.PLUGIN,
	spammer.PLUGIN,
	broadcastData.PLUGIN,
	getTransactionTrytesByHash.PLUGIN,
	getTransactionObjectsByHash.PLUGIN,
	findTransactionHashes.PLUGIN,
	getNeighbors.PLUGIN,
)
