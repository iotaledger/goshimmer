package drng

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPIDRNGEndpoint"

type dependencies struct {
	dig.In

	Server       *echo.Echo
	DrngInstance *drng.DRNG `optional:"true"`
	Tangle       *tangle.Tangle
}

var (
	// Plugin is the plugin instance of the web API DRNG endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	if deps.DrngInstance == nil {
		return
	}
	deps.Server.POST("drng/collectiveBeacon", collectiveBeaconHandler)
	deps.Server.GET("drng/info/committee", committeeHandler)
	deps.Server.GET("drng/info/randomness", randomnessHandler)
}
