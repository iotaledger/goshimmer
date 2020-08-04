package staticpeering

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi/staticpeering/removepeers"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/staticpeering/addpeers"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API staticpeering endpoint plugin.
const PluginName = "WebAPI staticpeering Endpoint"

var (
	// plugin is the plugin instance of the web API staticpeering endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

func configure(plugin *node.Plugin) {
	webapi.Server().GET("staticpeering/addPeers", addpeers.Handler)
	webapi.Server().GET("staticpeering/removePeers", removepeers.Handler)
}

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}
