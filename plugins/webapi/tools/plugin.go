package tools

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
)

// PluginName is the name of the web API tools endpoint plugin.
const PluginName = "WebAPI tools Endpoint"

const (
	routeDiagnostics = "tools/diagnostic"
	// RouteDiagnosticMessages is the API route for message diagnostics
	RouteDiagnosticMessages = routeDiagnostics + "/messages"
	// RouteDiagnosticsFirstWeakMessageReferences is the API route for first weak message diagnostics
	RouteDiagnosticsFirstWeakMessageReferences = RouteDiagnosticMessages + "/firstweakreferences"
	// RouteDiagnosticsMessageRank is the API route for message diagnostics with a rank filter
	RouteDiagnosticsMessageRank = RouteDiagnosticMessages + "/rank/:rank"
	// RouteDiagnosticsUtxoDag is the API route for Utxo Dag diagnostics
	RouteDiagnosticsUtxoDag = routeDiagnostics + "/utxodag"
	// RouteDiagnosticsBranches is the API route for branches diagnostics
	RouteDiagnosticsBranches = routeDiagnostics + "/branches"
	// RouteDiagnosticsLazyBookedBranches is the API route for booked branches diagnostics
	RouteDiagnosticsLazyBookedBranches = RouteDiagnosticsBranches + "/lazybooked"
	// RouteDiagnosticsInvalidBranches is the API route for invalid branches diagnostics
	RouteDiagnosticsInvalidBranches = RouteDiagnosticsBranches + "/invalid"
	// RouteDiagnosticsTips is the API route for tips diagnostics
	RouteDiagnosticsTips = routeDiagnostics + "/tips"
	// RouteDiagnosticsStrongTips is the API route for strong tips diagnostics
	RouteDiagnosticsStrongTips = RouteDiagnosticsTips + "/strong"
	// RouteDiagnosticsWeakTips is the API route for weak tips diagnostics
	RouteDiagnosticsWeakTips = RouteDiagnosticsTips + "/weak"
	// RouteDiagnosticsDRNG is the API route for DRNG diagnostics
	RouteDiagnosticsDRNG = routeDiagnostics + "/drng"
)

type dependencies struct {
	dig.In

	Server *echo.Echo
}

var (
	// Plugin is the plugin instance of the web API tools endpoint plugin.
	Plugin *node.Plugin
	deps   dependencies
)

func init() {
	Plugin = node.NewPlugin(PluginName, node.Disabled, configure)
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	if err := dependencyinjection.Container.Invoke(message.Invoke); err != nil {
		plugin.LogError(err)
	}
	if err := dependencyinjection.Container.Invoke(drng.Invoke); err != nil {
		plugin.LogError(err)
	}

	deps.Server.GET("tools/message/pastcone", message.PastconeHandler)
	deps.Server.GET("tools/message/missing", message.MissingHandler)
	deps.Server.GET("tools/message/approval", message.ApprovalHandler)
	deps.Server.GET("tools/message/orphanage", message.OrphanageHandler)
	deps.Server.GET(RouteDiagnosticMessages, message.DiagnosticMessagesHandler)
	deps.Server.GET(RouteDiagnosticsFirstWeakMessageReferences, message.DiagnosticMessagesOnlyFirstWeakReferencesHandler)
	deps.Server.GET(RouteDiagnosticsMessageRank, message.DiagnosticMessagesRankHandler)
	deps.Server.GET(RouteDiagnosticsUtxoDag, message.DiagnosticUTXODAGHandler)
	deps.Server.GET(RouteDiagnosticsBranches, message.DiagnosticBranchesHandler)
	deps.Server.GET(RouteDiagnosticsLazyBookedBranches, message.DiagnosticLazyBookedBranchesHandler)
	deps.Server.GET(RouteDiagnosticsInvalidBranches, message.DiagnosticInvalidBranchesHandler)
	deps.Server.GET(RouteDiagnosticsTips, message.TipsDiagnosticHandler)
	deps.Server.GET(RouteDiagnosticsStrongTips, message.StrongTipsDiagnosticHandler)
	deps.Server.GET(RouteDiagnosticsWeakTips, message.WeakTipsDiagnosticHandler)
	deps.Server.GET(RouteDiagnosticsDRNG, drng.DiagnosticDRNGMessagesHandler)
}
