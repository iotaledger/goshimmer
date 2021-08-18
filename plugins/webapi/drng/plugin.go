package drng

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPI DRNG Endpoint"

type dependencies struct {
	dig.In

	Server       *echo.Echo
	DrngInstance *drng.DRNG
	Tangle       *tangle.Tangle
}

var (
	// Plugin is the plugin instance of the web API DRNG endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies
)

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	deps.Server.POST("drng/collectiveBeacon", collectiveBeaconHandler)
	deps.Server.GET("drng/info/committee", committeeHandler)
	deps.Server.GET("drng/info/randomness", randomnessHandler)
}
