package pow

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the PoW plugin.
const PluginName = "PoW"

var (
	// Plugin is the plugin instance of the port check plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, run)
)

func run(*node.Plugin) {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)

	if node.IsSkipped(messagelayer.Plugin) {
		log.Infof("%s is disabled; skipping %s\n", messagelayer.PluginName, PluginName)
		return
	}

	messagelayer.MessageParser.AddBytesFilter(&powFilter{})
	messagelayer.MessageFactory.SetWorker(messagefactory.WorkerFunc(DoPOW))
}
