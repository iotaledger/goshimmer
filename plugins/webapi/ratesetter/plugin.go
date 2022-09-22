package ratesetter

import (
	"github.com/iotaledger/hive.go/core/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/protocol"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPIRateSetterEndpoint"

type dependencies struct {
	dig.In

	Server   *echo.Echo
	Protocol *protocol.Protocol
}

var (
	// Plugin is the plugin instance of the web API info endpoint plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("ratesetter", getRateSetterEstimate)
}

func getRateSetterEstimate(c echo.Context) error {
	// TODO: finish when ratesetter is available
	//return c.JSON(http.StatusOK, jsonmodels.RateSetter{
	//	Rate:     deps.Tangle.RateSetter.Rate(),
	//	Size:     deps.Tangle.RateSetter.Size(),
	//	Estimate: deps.Tangle.RateSetter.Estimate(),
	//})
	return nil
}
