package pow

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

// PluginName is the name of the PoW plugin.
const PluginName = "PoW"

var (
	// Plugin is the plugin instance of the PoW plugin.
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In

	Tangle             *tangle.Tangle
	MessagelayerPlugin *node.Plugin `name:"messagelayer"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}

	if node.IsSkipped(deps.MessagelayerPlugin) {
		log.Infof("%s is disabled; skipping %s\n", deps.MessagelayerPlugin.Name, PluginName)
		return
	}

	// assure that the PoW worker is initialized
	worker := Worker()

	log.Infof("%s started: difficult=%d", PluginName, difficulty)

	deps.Tangle.Parser.AddBytesFilter(tangle.NewPowFilter(worker, difficulty))
	deps.Tangle.MessageFactory.SetWorker(tangle.WorkerFunc(DoPOW))
	deps.Tangle.MessageFactory.SetTimeout(timeout)
}
