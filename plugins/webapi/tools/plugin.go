package tools

import (
	"sync"

	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
)

// PluginName is the name of the web API tools endpoint plugin.
const PluginName = "WebAPI tools Endpoint"

const (
	routeDiagnostics = "tools/diagnostic"
	// API route for message diagnostics
	RouteDiagnosticMessages = routeDiagnostics + "/messages"
	// API route for first weak message diagnostics
	RouteDiagnosticsFirstWeakMessageReferences = RouteDiagnosticMessages + "/firstweakreferences"
	// API route for message diagnostics with a rank filter
	RouteDiagnosticsMessageRank = RouteDiagnosticMessages + "/rank/:rank"
	// API route for Utxo Dag diagnostics
	RouteDiagnosticsUtxoDag = routeDiagnostics + "/utxodag"
	// API route for branches diagnostics
	RouteDiagnosticsBranches = routeDiagnostics + "/branches"
	// API route for booked branches diagnostics
	RouteDiagnosticsLazyBookedBranches = RouteDiagnosticsBranches + "/lazybooked"
	// API route for invalid branches diagnostics
	RouteDiagnosticsInvalidBranches = RouteDiagnosticsBranches + "/invalid"
	// API route for tips diagnostics
	RouteDiagnosticsTips = routeDiagnostics + "/tips"
	// API route for strong tips diagnostics
	RouteDiagnosticsStrongTips = RouteDiagnosticsTips + "/strong"
	// API route for weak tips diagnostics
	RouteDiagnosticsWeakTips = RouteDiagnosticsTips + "/weak"
	// API route for DRNG diagnostics
	RouteDiagnosticsDRNG = routeDiagnostics + "/drng"
)

var (
	// plugin is the plugin instance of the web API tools endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().GET("tools/message/pastcone", message.PastconeHandler)
	webapi.Server().GET("tools/message/missing", message.MissingHandler)
	webapi.Server().GET("tools/message/approval", message.ApprovalHandler)
	webapi.Server().GET("tools/message/orphanage", message.OrphanageHandler)
	webapi.Server().GET(RouteDiagnosticMessages, message.DiagnosticMessagesHandler)
	webapi.Server().GET(RouteDiagnosticsFirstWeakMessageReferences, message.DiagnosticMessagesOnlyFirstWeakReferencesHandler)
	webapi.Server().GET(RouteDiagnosticsMessageRank, message.DiagnosticMessagesRankHandler)
	webapi.Server().GET(RouteDiagnosticsUtxoDag, message.DiagnosticUTXODAGHandler)
	webapi.Server().GET(RouteDiagnosticsBranches, message.DiagnosticBranchesHandler)
	webapi.Server().GET(RouteDiagnosticsLazyBookedBranches, message.DiagnosticLazyBookedBranchesHandler)
	webapi.Server().GET(RouteDiagnosticsInvalidBranches, message.DiagnosticInvalidBranchesHandler)
	webapi.Server().GET(RouteDiagnosticsTips, message.TipsDiagnosticHandler)
	webapi.Server().GET(RouteDiagnosticsStrongTips, message.StrongTipsDiagnosticHandler)
	webapi.Server().GET(RouteDiagnosticsWeakTips, message.WeakTipsDiagnosticHandler)
	webapi.Server().GET(RouteDiagnosticsDRNG, drng.DiagnosticDRNGMessagesHandler)
}
