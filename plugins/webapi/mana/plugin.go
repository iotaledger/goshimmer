package mana

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API mana endpoint plugin.
const PluginName = "WebAPI Mana Endpoint"

var (
	// plugin is the plugin instance of the web API mana endpoint plugin.
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

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().GET("mana", getMana)
	webapi.Server().GET("mana/all", getAllMana)
	webapi.Server().GET("/mana/access/nhighest", getNHighestAccess)
	webapi.Server().GET("/mana/consensus/nhighest", getNHighestConsensus)
	webapi.Server().GET("/mana/percentile", getPercentile)
	webapi.Server().GET("/mana/access/online", getOnlineAccess)
	webapi.Server().GET("/mana/consensus/online", getOnlineConsensus)
}
