package pow

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the PoW plugin.
const PluginName = "PoW"

var (
	// Plugin is the plugin instance of the PoW plugin.
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

func configure(*node.Plugin) {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)

	if node.IsSkipped(messagelayer.Plugin()) {
		log.Infof("%s is disabled; skipping %s\n", messagelayer.PluginName, PluginName)
		return
	}

	// assure that the PoW worker is initialized
	worker := Worker()

	log.Infof("%s started: difficult=%d", PluginName, difficulty)

	messagelayer.Tangle().Parser.AddBytesFilter(tangle.NewPowFilter(worker, difficulty))
	messagelayer.Tangle().MessageFactory.SetWorker(tangle.WorkerFunc(DoPOW))
}
