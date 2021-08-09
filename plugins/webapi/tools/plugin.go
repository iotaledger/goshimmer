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
