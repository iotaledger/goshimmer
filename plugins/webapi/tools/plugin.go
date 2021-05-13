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
	RouteDiagnosticMessages                    = "tools/diagnostic/messages"
	RouteDiagnosticsFirstWeakMessageReferences = RouteDiagnosticMessages + "/firstweakreferences"
	RouteDiagnosticsMessageRank                = RouteDiagnosticMessages + "/rank/:rank"
	RouteDiagnosticsUtxoDag                    = "tools/diagnostic/utxodag"
	RouteDiagnosticsBranches                   = "tools/diagnostic/branches"
	RouteDiagnosticsLazyBookedBranches         = RouteDiagnosticsBranches + "/lazybooked"
	RouteDiagnosticsInvalidBranches            = RouteDiagnosticsBranches + "/invalid"
	RouteDiagnosticsTips                       = "tools/diagnostic/tips"
	RouteDiagnosticsStrongTips                 = RouteDiagnosticsTips + "/strong"
	RouteDiagnosticsWeakTips                   = RouteDiagnosticsTips + "/weak"
	RouteDiagnosticsDRNG                       = "tools/diagnostic/drng"
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
