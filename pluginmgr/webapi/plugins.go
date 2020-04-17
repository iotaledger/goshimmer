package webapi

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi/data"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/info"
	"github.com/iotaledger/goshimmer/plugins/webapi/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	webapi.PLUGIN,
	webauth.PLUGIN,
	spammer.PLUGIN,
	data.PLUGIN,
	drng.PLUGIN,
	message.PLUGIN,
	autopeering.PLUGIN,
	info.Plugin,
)
