package pow

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the PoW plugin.
const PluginName = "PoW"

var (
	// Plugin is the plugin instance of the port check plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, run)
)

func run(*node.Plugin) {
	messagelayer.MessageParser.AddMessageFilter(&powFilter{})
	messagelayer.MessageFactory.SetWorker(messagefactory.WorkerFunc(DoPOW))
}
