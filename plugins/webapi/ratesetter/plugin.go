package ratesetter

import (
	"net/http"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the web API info endpoint plugin.
const PluginName = "WebAPIRateSetterEndpoint"

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
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
	return c.JSON(http.StatusOK, jsonmodels.RateSetter{
		Rate:     deps.Tangle.RateSetter.Rate(),
		Size:     deps.Tangle.RateSetter.Size(),
		Estimate: deps.Tangle.RateSetter.Estimate(),
	})
}
