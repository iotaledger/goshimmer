package mana

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"
)

// PluginName is the name of the web API mana endpoint plugin.
const PluginName = "WebAPIManaEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Local  *peer.Local
}

var (
	// Plugin is the plugin instance of the web API mana endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("mana", getManaHandler)
	deps.Server.GET("mana/all", getAllManaHandler)
	deps.Server.GET("/mana/access/nhighest", getNHighestAccessHandler)
	deps.Server.GET("/mana/consensus/nhighest", getNHighestConsensusHandler)
	deps.Server.GET("/mana/percentile", getPercentileHandler)
	deps.Server.GET("/mana/access/online", getOnlineAccessHandler)
	deps.Server.GET("/mana/consensus/online", getOnlineConsensusHandler)
	deps.Server.GET("/mana/pending", GetPendingMana)
	deps.Server.GET("mana/allowedManaPledge", allowedManaPledgeHandler)
	deps.Server.GET("mana/delegated", GetDelegatedMana)
	deps.Server.GET("mana/delegated/outputs", GetDelegatedOutputs)
	// deps.Server.GET("/mana/consensus/past", getPastConsensusManaVectorHandler)
	// deps.Server.GET("/mana/consensus/logs", getEventLogsHandler)
	// deps.Server.GET("/mana/consensus/metadata", getPastConsensusVectorMetadataHandler)
}
