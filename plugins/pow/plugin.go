package pow

import (
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// PluginName is the name of the PoW plugin.
const PluginName = "POW"

var (
	// Plugin is the plugin instance of the PoW plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle           *tangleold.Tangle
	BlocklayerPlugin *node.Plugin `name:"blocklayer"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)

	if node.IsSkipped(deps.BlocklayerPlugin) {
		log.Infof("%s is disabled; skipping %s\n", deps.BlocklayerPlugin.Name, PluginName)
		return
	}

	// assure that the PoW worker is initialized
	worker := Worker()

	log.Infof("%s started: difficult=%d", PluginName, difficulty)

	deps.Tangle.Parser.AddBytesFilter(tangleold.NewPowFilter(worker, difficulty))
	deps.Tangle.BlockFactory.SetWorker(tangleold.WorkerFunc(DoPOW))
	deps.Tangle.BlockFactory.SetTimeout(timeout)
}
