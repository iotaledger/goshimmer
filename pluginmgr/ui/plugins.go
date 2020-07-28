package ui

import (
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/hive.go/node"
)

// PLUGINS is the list of ui plugins.
var PLUGINS = node.Plugins(
	dashboard.Plugin(),
)
