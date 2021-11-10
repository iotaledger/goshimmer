package drng

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the web API tools DRNG endpoint plugin.
const PluginName = "WebAPIToolsDRNGEndpoint"

var (
	// Plugin is the plugin instance of the web API tools messages endpoint plugin.
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle *tangle.Tangle
	Server *echo.Echo
}

const (
	routeDiagnostics = "tools/diagnostic"
	// RouteDiagnosticsDRNG is the API route for DRNG diagnostics.
	RouteDiagnosticsDRNG = routeDiagnostics + "/drng"
)

func configure(_ *node.Plugin) {
	deps.Server.GET(RouteDiagnosticsDRNG, DiagnosticDRNGMessagesHandler)
}
