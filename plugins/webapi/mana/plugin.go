package mana

import (
	"sync"

	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the web API mana endpoint plugin.
const PluginName = "WebAPI Mana Endpoint"

var (
	// plugin is the plugin instance of the web API mana endpoint plugin.
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
	webapi.Server().GET("mana", getManaHandler)
	webapi.Server().GET("mana/all", getAllManaHandler)
	webapi.Server().GET("/mana/access/nhighest", getNHighestAccessHandler)
	webapi.Server().GET("/mana/consensus/nhighest", getNHighestConsensusHandler)
	webapi.Server().GET("/mana/percentile", getPercentileHandler)
	webapi.Server().GET("/mana/access/online", getOnlineAccessHandler)
	webapi.Server().GET("/mana/consensus/online", getOnlineConsensusHandler)
	webapi.Server().GET("/mana/pending", GetPendingMana)
	webapi.Server().GET("/mana/consensus/past", getPastConsensusManaVectorHandler)
	webapi.Server().GET("/mana/consensus/logs", getEventLogsHandler)
	webapi.Server().GET("/mana/consensus/metadata", getPastConsensusVectorMetadataHandler)
}
