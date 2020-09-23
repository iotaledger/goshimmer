package webapi

import (
	"github.com/iotaledger/goshimmer/plugins/spammer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webauth"
	"github.com/iotaledger/hive.go/node"
)

// PLUGINS is the list of webapi plugins.
var PLUGINS = node.Plugins(
	webapi.Plugin(),
	webauth.Plugin(),
	spammer.Plugin(),
)
