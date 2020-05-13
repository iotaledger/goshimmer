package fpctest

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPI FPCTest Endpoint"

var (
	// Plugin is the plugin instance of the web API DRNG endpoint plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
)

func configure(_ *node.Plugin) {
	webapi.Server.POST("fpctest/broadcast", Handler)
}
