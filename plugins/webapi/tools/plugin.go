package tools

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message/missing"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message/pastcone"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/value/objects"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/value/tips"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API tools endpoint plugin.
const PluginName = "WebAPI tools Endpoint"

var (
	// plugin is the plugin instance of the web API tools endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().GET("tools/message/pastcone", pastcone.Handler)
	webapi.Server().GET("tools/message/missing", missing.Handler)
	webapi.Server().GET("tools/value/tips", tips.Handler)
	webapi.Server().GET("tools/value/objects", objects.Handler)
}
