package message

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API message endpoint plugin.
const PluginName = "WebAPI message Endpoint"

var (
	// plugin is the plugin instance of the web API message endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().POST("message/findById", findByIDHandler)
	webapi.Server().POST("message/sendPayload", sendPayloadHandler)
}
