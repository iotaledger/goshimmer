package tools

import (
	"sync"

	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/drng"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
	"github.com/iotaledger/goshimmer/plugins/webapi/tools/value"
)

// PluginName is the name of the web API tools endpoint plugin.
const PluginName = "WebAPI tools Endpoint"

var (
	// plugin is the plugin instance of the web API tools endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	webapi.Server().GET("tools/message/pastcone", message.PastconeHandler)
	webapi.Server().GET("tools/message/missing", message.MissingHandler)
	webapi.Server().GET("tools/message/approval", message.ApprovalHandler)
	webapi.Server().GET("tools/value/objects", value.ObjectsHandler)
	webapi.Server().GET("tools/message/orphanage", message.OrphanageHandler)

	webapi.Server().GET("tools/diagnostic/messages", message.DiagnosticMessagesHandler)
	webapi.Server().GET("tools/diagnostic/messages/firstweakreferences", message.DiagnosticMessagesOnlyFirstWeakReferencesHandler)
	webapi.Server().GET("tools/diagnostic/messages/rank/:rank", message.DiagnosticMessagesRankHandler)
	webapi.Server().GET("tools/diagnostic/utxodag", message.DiagnosticUTXODAGHandler)
	webapi.Server().GET("tools/diagnostic/branches", message.DiagnosticBranchesHandler)
	webapi.Server().GET("tools/diagnostic/branches/lazybooked", message.DiagnosticLazyBookedBranchesHandler)
	webapi.Server().GET("tools/diagnostic/branches/invalid", message.DiagnosticInvalidBranchesHandler)
	webapi.Server().GET("tools/diagnostic/tips", message.TipsDiagnosticHandler)
	webapi.Server().GET("tools/diagnostic/tips/strong", message.StrongTipsDiagnosticHandler)
	webapi.Server().GET("tools/diagnostic/tips/weak", message.WeakTipsDiagnosticHandler)
	webapi.Server().GET("tools/diagnostic/drng", drng.DiagnosticDRNGMessagesHandler)
}
