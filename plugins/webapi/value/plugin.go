package value

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/attachments"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/gettransactionbyid"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/sendtransaction"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/sendtransactionbyjson"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/testsendtxn"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/unspentoutputs"
	"github.com/iotaledger/hive.go/node"
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
	webapi.Server().GET("value/attachments", attachments.Handler)
	webapi.Server().POST("value/unspentOutputs", unspentoutputs.Handler)
	webapi.Server().POST("value/sendTransaction", sendtransaction.Handler)
	webapi.Server().POST("value/sendTransactionByJson", sendtransactionbyjson.Handler)
	webapi.Server().POST("value/testSendTxn", testsendtxn.Handler)
	webapi.Server().GET("value/transactionByID", gettransactionbyid.Handler)
}
