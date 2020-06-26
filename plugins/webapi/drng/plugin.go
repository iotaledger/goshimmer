package drng

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/collectivebeacon"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/info/committee"
	"github.com/iotaledger/goshimmer/plugins/webapi/drng/info/randomness"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPI DRNG Endpoint"

var (
	// plugin is the plugin instance of the web API DRNG endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().POST("drng/collectiveBeacon", collectivebeacon.Handler)
	webapi.Server().GET("drng/info/committee", committee.Handler)
	webapi.Server().GET("drng/info/randomness", randomness.Handler)
}
