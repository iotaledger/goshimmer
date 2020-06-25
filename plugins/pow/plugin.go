package pow

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser/builtinfilters"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the PoW plugin.
const PluginName = "PoW"

var (
	// Plugin is the plugin instance of the PoW plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, run)
)

func run(*node.Plugin) {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)

	if node.IsSkipped(messagelayer.Plugin()) {
		log.Infof("%s is disabled; skipping %s\n", messagelayer.PluginName, PluginName)
		return
	}

	// assure that the PoW worker is initialized
	worker := Worker()

	log.Infof("%s started: difficult=%d", PluginName, difficulty)

	messagelayer.MessageParser().AddBytesFilter(builtinfilters.NewPowFilter(worker, difficulty))
	messagelayer.MessageFactory().SetWorker(messagefactory.WorkerFunc(DoPOW))
}
