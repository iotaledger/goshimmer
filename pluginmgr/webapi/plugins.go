package webapi

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
	"github.com/iotaledger/goshimmer/plugins/webapi/data"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/healthz"
	"github.com/iotaledger/goshimmer/plugins/webapi/info"
	"github.com/iotaledger/goshimmer/plugins/webapi/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	webapi.Plugin,
	webauth.Plugin,
	spammer.Plugin,
	data.Plugin,
	drng.Plugin,
	healthz.Plugin,
	message.Plugin,
	autopeering.Plugin,
	info.Plugin,
	value.Plugin,
)
