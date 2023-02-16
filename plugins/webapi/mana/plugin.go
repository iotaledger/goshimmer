package mana

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/protocol"
)

// PluginName is the name of the web API mana endpoint plugin.
const PluginName = "WebAPIManaEndpoint"

type dependencies struct {
	dig.In

	Discovery *discover.Protocol `optional:"true"`
	Protocol  *protocol.Protocol
	Server    *echo.Echo
	Local     *peer.Local
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
}
