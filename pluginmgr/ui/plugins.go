package ui

import (
	"github.com/iotaledger/goshimmer/plugins/graph"
	"github.com/iotaledger/goshimmer/plugins/spa"
	"github.com/iotaledger/hive.go/node"
)

var PLUGINS = node.Plugins(
	spa.PLUGIN,
	graph.PLUGIN,
)
