package value

import (
	"sync"

	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// PluginName is the name of the web API DRNG endpoint plugin.
const PluginName = "WebAPI Value Endpoint"

var (
	// plugin is the plugin instance of the web API DRNG endpoint plugin.
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
	webapi.Server().GET("value/attachments", attachmentsHandler)
	webapi.Server().POST("value/unspentOutputs", unspentOutputsHandler)
	webapi.Server().POST("value/sendTransaction", sendTransactionHandler)
	webapi.Server().POST("value/sendTransactionByJson", sendTransactionByJSONHandler)
	webapi.Server().GET("value/transactionByID", getTransactionByIDHandler)
	webapi.Server().GET("value/allowedManaPledge", allowedManaPledgeHandler)
}
