package ui

import (
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	dashboard.Plugin,
)
