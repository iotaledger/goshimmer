package webapi

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/findMessageById"
	"github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	"github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	webapi.PLUGIN,
	webauth.PLUGIN,
	//gtta.PLUGIN,
	spammer.PLUGIN,
	broadcastData.PLUGIN,
	drng.PLUGIN,
	findMessageById.PLUGIN,
	getNeighbors.PLUGIN,
)
