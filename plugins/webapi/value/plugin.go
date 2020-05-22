package value

import (
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/attachments"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/gettransactionbyid"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/unspentoutputs"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPI Value Endpoint"

var (
	// Plugin is the plugin instance of the web API DRNG endpoint plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
)

func configure(_ *node.Plugin) {
	webapi.Server.GET("value/attachments", attachments.Handler)
	webapi.Server.POST("value/unspentOutputs", unspentoutputs.Handler)
	webapi.Server.GET("value/transactionByID", gettransactionbyid.Handler)
}
