package logger

import (
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the logger plugin.
const PluginName = "Logger"

// plugin is the plugin instance of the logger plugin.
var plugin = node.NewPlugin(PluginName, node.Enabled)

// Plugin gets the plugin instance
func Plugin() *node.Plugin {
	return plugin
}


// Init triggers the Init event.
func Init() {
	plugin.Events.Init.Trigger(plugin)
}

func init() {
	initFlags()

	plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		if err := logger.InitGlobalLogger(config.Node()); err != nil {
			panic(err)
		}
	}))
}
