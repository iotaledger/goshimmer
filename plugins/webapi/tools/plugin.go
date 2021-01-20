package tools

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/value"
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
	webapi.Server().GET("tools/message/pastcone", message.PastconeHandler)
	webapi.Server().GET("tools/message/missing", message.MissingHandler)
	webapi.Server().GET("tools/message/approval", message.ApprovalHandler)
	webapi.Server().GET("tools/value/tips", value.TipsHandler)
	webapi.Server().GET("tools/value/objects", value.ObjectsHandler)
}
